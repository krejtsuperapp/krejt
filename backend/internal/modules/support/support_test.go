package support

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"krejt.app/backend/internal/platform/db"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

func TestSupportFlow(t *testing.T) {
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
	svc := New(pool)
	newUser := func() principal.Actor {
		var id uuid.UUID
		if err := pool.QueryRow(ctx, `INSERT INTO users (phone_e164, full_name, locale) VALUES ($1, 'Test', 'sq') RETURNING id`, "+38340"+uuid.NewString()[:6]).Scan(&id); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			pool.Exec(context.Background(), `UPDATE support_tickets SET assigned_to = NULL WHERE assigned_to = $1`, id)
			pool.Exec(context.Background(), `DELETE FROM safety_reports WHERE reporter_id = $1`, id)
			pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id)
		})
		return principal.Actor{UserID: id, IP: "203.0.113.3"}
	}
	user, agent, other := newUser(), newUser(), newUser()

	if _, err := svc.Create(ctx, user, CreateInput{Category: "weather", Subject: "x", Body: "y"}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("validimi: %v", err)
	}
	if _, err := svc.Create(ctx, user, CreateInput{Category: "ride", Subject: "Udhëtim i gabuar", Body: "Shoferi mori rrugë tjetër", RideID: &[]uuid.UUID{uuid.New()}[0]}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("ride_id i huaj: %v", err)
	}
	tk, err := svc.Create(ctx, user, CreateInput{Category: "payment", Subject: "  Pagesa   dyfish ", Body: "U paguan dy herë."})
	if err != nil || tk.Priority != "high" || tk.Subject != "Pagesa dyfish" || tk.Status != "open" {
		t.Fatalf("create: %+v err=%v", tk, err)
	}
	if _, err := svc.Get(ctx, other, tk.ID); !errors.Is(err, httpx.ErrNotFound) {
		t.Fatalf("BOLA: %v", err)
	}
	got, err := svc.Get(ctx, user, tk.ID)
	if err != nil || len(got.Messages) != 1 || got.Messages[0].AuthorRole != "user" || got.AssignedTo != nil {
		t.Fatalf("get: %+v err=%v", got, err)
	}

	// agjenti: radha (urgent para high), kontekst, përgjigje → pending_user, njoftim (ngjarje)
	q, err := svc.Queue(ctx, QueueFilter{})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, x := range q {
		if x.ID == tk.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("tiketa duhej në radhë")
	}
	at, err := svc.AgentGet(ctx, agent, tk.ID)
	if err != nil || at.Context == nil || at.Context.UserPhone == nil || at.Context.UserName == nil {
		t.Fatalf("agent get: %+v err=%v", at, err)
	}
	if _, err := svc.AgentReply(ctx, agent, tk.ID, "Po e shqyrtojmë, kthehemi brenda ditës."); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Get(ctx, user, tk.ID)
	if got.Status != "pending_user" || len(got.Messages) != 2 {
		t.Fatalf("pas përgjigjes: %+v", got)
	}
	var evs int
	pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND event_type = 'SupportTicketReplied'`, tk.ID.String()).Scan(&evs)
	if evs != 1 {
		t.Fatalf("ngjarja e përgjigjes: %d", evs)
	}
	// përdoruesi përgjigjet → open; agjenti zgjidh → resolved; mesazh pas mbylljes → refuzohet
	if _, err := svc.Reply(ctx, user, tk.ID, "Faleminderit"); err != nil {
		t.Fatal(err)
	}
	got, _ = svc.Get(ctx, user, tk.ID)
	if got.Status != "open" {
		t.Fatalf("pas përgjigjes së përdoruesit: %s", got.Status)
	}
	st := "resolved"
	if _, err := svc.AgentUpdate(ctx, agent, tk.ID, AgentUpdate{Status: &st}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Close(ctx, user, tk.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reply(ctx, user, tk.ID, "edhe diçka"); !errors.Is(err, ErrTicketClosed) {
		t.Fatalf("pas mbylljes: %v", err)
	}

	// SOS → tiketë urgjente + raport + ngjarje
	rep, err := svc.ReportSafety(ctx, user, SafetyInput{Kind: "sos", Lat: 42.66, Lng: 21.16})
	if err != nil || rep.Status != "open" {
		t.Fatalf("sos: %+v err=%v", rep, err)
	}
	var prio, subj string
	pool.QueryRow(ctx, `SELECT priority, subject FROM support_tickets WHERE id = $1`, rep.TicketID).Scan(&prio, &subj)
	if prio != "urgent" || subj != "SOS" {
		t.Fatalf("tiketa e SOS: %s %s", prio, subj)
	}
	pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND event_type = 'SafetyReportCreated'`, rep.ID.String()).Scan(&evs)
	if evs != 1 {
		t.Fatalf("ngjarja e SOS: %d", evs)
	}
	q, _ = svc.Queue(ctx, QueueFilter{Priority: "urgent"})
	if len(q) == 0 || q[0].Priority != "urgent" {
		t.Fatalf("radha urgjente: %+v", q)
	}
	closed := "closed"
	if _, err := svc.AgentUpdate(ctx, agent, rep.TicketID, AgentUpdate{Status: &closed}); err != nil {
		t.Fatal(err)
	}
	var rstatus string
	pool.QueryRow(ctx, `SELECT status FROM safety_reports WHERE id = $1`, rep.ID).Scan(&rstatus)
	if rstatus != "closed" {
		t.Fatalf("raporti pas mbylljes: %s", rstatus)
	}
	if _, err := svc.ReportSafety(ctx, user, SafetyInput{Kind: "teleport"}); !errors.Is(err, httpx.ErrValidation) {
		t.Fatalf("kind i pavlefshëm: %v", err)
	}
}
