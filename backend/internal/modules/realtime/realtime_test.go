package realtime

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/platform/principal"
	rt "krejt.app/backend/internal/platform/providers/realtime"
)

type fakePub struct{ got []string }

func (f *fakePub) Publish(_ context.Context, channel string, data any) error {
	f.got = append(f.got, channel)
	return nil
}

func ev(t *testing.T, typ string, payload map[string]any) events.Event {
	t.Helper()
	raw, _ := json.Marshal(payload)
	return events.Event{ID: uuid.New(), EventType: typ, OccurredAt: time.Now(), Payload: raw}
}

func TestHandleRoutesEventsToChannels(t *testing.T) {
	f := &fakePub{}
	s := New(nil, f, nil)
	ride, drv := uuid.New(), uuid.New()
	ctx := context.Background()
	_ = s.Handle(ctx, ev(t, "RideOffered", map[string]any{"ride_id": ride, "driver_id": drv, "offer_id": uuid.New()}))
	_ = s.Handle(ctx, ev(t, "RideAssigned", map[string]any{"ride_id": ride, "driver_id": drv}))
	_ = s.Handle(ctx, ev(t, "RideCancelled", map[string]any{"ride_id": ride, "driver_id": drv, "by": "customer"}))
	_ = s.Handle(ctx, ev(t, "RideRequested", map[string]any{"ride_id": ride, "attempt": 1}))
	_ = s.Handle(ctx, ev(t, "RideRequested", map[string]any{"ride_id": ride, "reassign": true}))
	_ = s.Handle(ctx, ev(t, "UserCreated", map[string]any{"user_id": uuid.New()}))
	want := []string{DriverChannel(drv), RideChannel(ride), RideChannel(ride), DriverChannel(drv), RideChannel(ride)}
	if len(f.got) != len(want) {
		t.Fatalf("kanalet = %v, pritej %v", f.got, want)
	}
	for i := range want {
		if f.got[i] != want[i] {
			t.Fatalf("kanali %d = %s, pritej %s", i, f.got[i], want[i])
		}
	}
}

func TestAuthorizeChannels(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := db.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	iss, _ := rt.NewTokenIssuer("development", "", logx.New("test", "development"))
	s := New(pool, &fakePub{}, iss)

	me, other := principal.Actor{UserID: uuid.New()}, principal.Actor{UserID: uuid.New()}
	for _, c := range []struct {
		actor principal.Actor
		ch    string
		want  bool
	}{
		{me, UserChannel(me.UserID), true}, {other, UserChannel(me.UserID), false},
		{me, DriverChannel(me.UserID), true}, {me, DriverChannel(other.UserID), false},
		{me, "ride:not-a-uuid", false}, {me, "admin:" + me.UserID.String(), false}, {me, "nocolon", false},
		{me, RideChannel(uuid.New()), false}, // udhëtim që s'ekziston
	} {
		ok, err := s.Authorize(ctx, c.actor, c.ch)
		if err != nil || ok != c.want {
			t.Errorf("%s për %s: ok=%v err=%v (pritej %v)", c.ch, c.actor.UserID, ok, err, c.want)
		}
	}
}
