package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
	"krejt.app/backend/internal/platform/providers/push"
)

type fakePush struct {
	sent    []push.Message
	invalid map[string]bool
}

func (f *fakePush) Send(_ context.Context, m push.Message) (push.Result, error) {
	if f.invalid[m.Token] {
		return push.Result{InvalidToken: true}, errors.New("push: fcm: UNREGISTERED")
	}
	f.sent = append(f.sent, m)
	return push.Result{ProviderMessageID: "fake/" + uuid.NewString()}, nil
}

// --- test integrimi (kërkon TEST_DATABASE_URL) --------------------------------

func TestHandleInboxPushPreferencesAndIdempotency(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	fp := &fakePush{invalid: map[string]bool{}}
	svc := New(pool, fp)

	var uid uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, full_name, locale) VALUES ($1, 'Arta', 'de') RETURNING id`, "+38348"+uuid.NewString()[:6]).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, uid)
	a := principal.Actor{UserID: uid, SessionID: uuid.New()}

	// dy pajisje: njëra gjermanisht, tjetra anglisht; njëra do të dalë e pavlefshme
	deadToken := "dead-" + uuid.NewString() + uuid.NewString()
	liveToken := "live-" + uuid.NewString() + uuid.NewString()
	fp.invalid[deadToken] = true
	if err := svc.RegisterToken(ctx, a, RegisterTokenInput{Platform: "android", Token: liveToken, Locale: "en"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.RegisterToken(ctx, a, RegisterTokenInput{Platform: "ios", Token: deadToken}); err != nil { // locale → sq
		t.Fatal(err)
	}
	if err := svc.RegisterToken(ctx, a, RegisterTokenInput{Platform: "web", Token: "short"}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("token i shkurtër: %v", err)
	}

	rideID := uuid.New()
	raw, _ := json.Marshal(map[string]any{"ride_id": rideID, "customer_id": uid, "driver_id": uuid.New()})
	e := events.Event{ID: uuid.New(), EventType: "RideAssigned", AggregateType: "ride", AggregateID: rideID.String(), OccurredAt: time.Now(), Payload: raw}
	if err := svc.Handle(ctx, e); err != nil {
		t.Fatal(err)
	}
	if len(fp.sent) != 1 || fp.sent[0].Token != liveToken || fp.sent[0].Title != "Driver assigned" || fp.sent[0].Data["deep_link"] != "krejt://rides/"+rideID.String() {
		t.Fatalf("push: %+v", fp.sent)
	}
	// ridorëzim i së njëjtës ngjarje (SQS at-least-once) → asnjë push i dytë, asnjë rresht i dytë
	if err := svc.Handle(ctx, e); err != nil {
		t.Fatal(err)
	}
	if len(fp.sent) != 1 {
		t.Fatalf("push i dyfishuar: %d", len(fp.sent))
	}
	inbox, err := svc.List(ctx, a, nil, 10)
	if err != nil || len(inbox.Items) != 1 || inbox.Unread != 1 || inbox.Items[0].TitleKey != "notif.ride.assigned.title" {
		t.Fatalf("kutia: %+v err=%v", inbox, err)
	}
	var invalidAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT invalid_at FROM push_tokens WHERE token = $1`, deadToken).Scan(&invalidAt); err != nil || invalidAt == nil {
		t.Fatalf("token-i i vdekur duhej shënuar i pavlefshëm: %v %v", invalidAt, err)
	}
	var sent, failed int
	pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='sent'), count(*) FILTER (WHERE status='failed') FROM notification_deliveries d JOIN notifications n ON n.id = d.notification_id WHERE n.user_id = $1`, uid).Scan(&sent, &failed)
	if sent != 1 || failed != 1 {
		t.Fatalf("gjurma e dorëzimit: sent=%d failed=%d", sent, failed)
	}

	// preferenca: promotions push OFF → kutia po, push jo; security nuk çaktivizohet dot (kontroll DB)
	if _, err := pool.Exec(ctx, `INSERT INTO notification_preferences (user_id, category, push, email, sms) VALUES ($1, 'rides', false, true, false)`, uid); err != nil {
		t.Fatal(err)
	}
	raw2, _ := json.Marshal(map[string]any{"ride_id": rideID, "customer_id": uid})
	if err := svc.Handle(ctx, events.Event{ID: uuid.New(), EventType: "RideDriverArrived", AggregateType: "ride", AggregateID: rideID.String(), OccurredAt: time.Now(), Payload: raw2}); err != nil {
		t.Fatal(err)
	}
	if len(fp.sent) != 1 {
		t.Fatalf("push me preferencë OFF: %d", len(fp.sent))
	}
	if _, err := pool.Exec(ctx, `INSERT INTO notification_preferences (user_id, category, push) VALUES ($1, 'security', false)`, uid); err == nil {
		t.Fatal("security.push=false duhej refuzuar nga DB")
	}
	inbox, _ = svc.List(ctx, a, nil, 10)
	if len(inbox.Items) != 2 || inbox.Unread != 2 {
		t.Fatalf("kutia 2: %+v", inbox)
	}
	if err := svc.MarkRead(ctx, a, inbox.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkRead(ctx, principal.Actor{UserID: uuid.New()}, inbox.Items[0].ID); err == nil {
		t.Fatal("BOLA: njoftimi i tjetërkujt")
	}
	if err := svc.MarkAllRead(ctx, a); err != nil {
		t.Fatal(err)
	}
	if inbox, _ = svc.List(ctx, a, nil, 10); inbox.Unread != 0 {
		t.Fatalf("unread pas read-all: %d", inbox.Unread)
	}
	if err := svc.RemoveToken(ctx, a, liveToken); err != nil {
		t.Fatal(err)
	}
}
