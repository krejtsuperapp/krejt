// Package support — mbështetja (§36): tiketa me mesazhe (klient/shofer ↔ Support), raportet e
// sigurisë (SOS → tiketë urgjente + ngjarje për Operacionet), përgjigjet e Support-it → njoftim.
// Support-i sheh kontekstin e nevojshëm (udhëtimi i lidhur), jo të dhëna të tjera të përdoruesit.
package support

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/platform/principal"
)

var (
	ErrTicketClosed = &httpx.APIError{Code: "TICKET_CLOSED", MessageKey: "errors.support.ticket_closed", HTTPStatus: http.StatusConflict}
)

var Categories = []string{"ride", "order", "payment", "refund", "account", "safety", "other"}
var SafetyKinds = []string{"sos", "unsafe_driving", "harassment", "accident", "vehicle_issue", "other"}

type Ticket struct {
	ID            uuid.UUID  `json:"id"`
	UserID        uuid.UUID  `json:"user_id"`
	Category      string     `json:"category"`
	Subject       string     `json:"subject"`
	Status        string     `json:"status"`
	Priority      string     `json:"priority"`
	RideID        *uuid.UUID `json:"ride_id"`
	AssignedTo    *uuid.UUID `json:"assigned_to,omitempty"`
	LastMessageAt time.Time  `json:"last_message_at"`
	CreatedAt     time.Time  `json:"created_at"`
	ResolvedAt    *time.Time `json:"resolved_at"`
	Messages      []Message  `json:"messages,omitempty"`
	Context       *Context   `json:"context,omitempty"`
}

type Message struct {
	ID         uuid.UUID `json:"id"`
	AuthorRole string    `json:"author_role"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"created_at"`
}

// Context — çfarë i duhet Support-it për tiketën (pa ekspozuar më shumë se duhet).
type Context struct {
	UserPhone  *string      `json:"user_phone,omitempty"`
	UserName   *string      `json:"user_name,omitempty"`
	UserLocale string       `json:"user_locale,omitempty"`
	Ride       *RideSummary `json:"ride,omitempty"`
}

type RideSummary struct {
	ID               uuid.UUID  `json:"id"`
	State            string     `json:"state"`
	CategoryID       string     `json:"category"`
	PriceQuotedMinor int64      `json:"price_quoted_minor"`
	PriceFinalMinor  *int64     `json:"price_final_minor"`
	PaymentMethod    string     `json:"payment_method"`
	PaymentStatus    string     `json:"payment_status"`
	DriverID         *uuid.UUID `json:"driver_id"`
	RequestedAt      time.Time  `json:"requested_at"`
	CompletedAt      *time.Time `json:"completed_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

const ticketCols = `id, user_id, category, subject, status, priority, ride_id, assigned_to, last_message_at, created_at, resolved_at`

func scanTicket(row pgx.Row) (*Ticket, error) {
	var t Ticket
	if err := row.Scan(&t.ID, &t.UserID, &t.Category, &t.Subject, &t.Status, &t.Priority, &t.RideID, &t.AssignedTo, &t.LastMessageAt, &t.CreatedAt, &t.ResolvedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

// --- klienti / shoferi -------------------------------------------------------------

type CreateInput struct {
	Category string     `json:"category"`
	Subject  string     `json:"subject"`
	Body     string     `json:"body"`
	RideID   *uuid.UUID `json:"ride_id"`
	// Një tiketë flet për një gjë të vetme; jepet së shumti njëra prej këtyre.
	OrderID   *uuid.UUID `json:"order_id"`
	ParcelID  *uuid.UUID `json:"parcel_id"`
	RequestID *uuid.UUID `json:"request_id"`
}

// subject — referenca e vetme e lejuar, e verifikuar se i takon vërtet këtij përdoruesi.
// Pa verifikim, kushdo mund të lidhte tiketën e vet me porosinë e dikujt tjetër dhe agjenti do të
// lexonte të dhëna që nuk i takojnë raportuesit.
func (in CreateInput) subject() (col string, id *uuid.UUID, owns string, n int) {
	if in.RideID != nil {
		col, id, owns, n = "ride_id", in.RideID, "SELECT EXISTS (SELECT 1 FROM rides WHERE id = $1 AND (customer_id = $2 OR driver_id = $2))", n+1
	}
	if in.OrderID != nil {
		col, id, owns, n = "order_id", in.OrderID, "SELECT EXISTS (SELECT 1 FROM orders WHERE id = $1 AND (customer_id = $2 OR courier_id = $2))", n+1
	}
	if in.ParcelID != nil {
		col, id, owns, n = "parcel_id", in.ParcelID, "SELECT EXISTS (SELECT 1 FROM parcels WHERE id = $1 AND (customer_id = $2 OR courier_id = $2))", n+1
	}
	if in.RequestID != nil {
		col, id, owns, n = "service_request_id", in.RequestID, "SELECT EXISTS (SELECT 1 FROM service_requests WHERE id = $1 AND (customer_id = $2 OR provider_id = $2))", n+1
	}
	return col, id, owns, n
}

func normText(s string, max int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) > max {
		s = string([]rune(s)[:max])
	}
	return s
}

func (s *Service) Create(ctx context.Context, a principal.Actor, in CreateInput) (*Ticket, error) {
	fields := map[string]string{}
	in.Category = strings.ToLower(strings.TrimSpace(in.Category))
	if !contains(Categories, in.Category) {
		fields["category"] = "invalid"
	}
	in.Subject = strings.Join(strings.Fields(in.Subject), " ")
	if n := utf8.RuneCountInString(in.Subject); n < 3 || n > 120 {
		fields["subject"] = "invalid"
	}
	in.Body = normText(in.Body, 2000)
	if utf8.RuneCountInString(in.Body) < 3 {
		fields["body"] = "required"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	col, subjectID, owns, refs := in.subject()
	if refs > 1 {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"subject": "too_many"})
	}
	if subjectID != nil {
		var ok bool
		if err := s.pool.QueryRow(ctx, owns, *subjectID, a.UserID).Scan(&ok); err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpx.ErrValidation.WithFields(map[string]string{col: "not_yours"})
		}
	}
	priority := "normal"
	if in.Category == "safety" {
		priority = "urgent"
	} else if in.Category == "payment" || in.Category == "refund" {
		priority = "high"
	}
	var out *Ticket
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Kolona e referencës zgjidhet nga subjekti; pa referencë, tiketa mbetet e përgjithshme.
		refCol, refVal := "ride_id", (*uuid.UUID)(nil)
		if subjectID != nil {
			refCol, refVal = col, subjectID
		}
		t, err := scanTicket(tx.QueryRow(ctx, `INSERT INTO support_tickets (user_id, category, subject, priority, `+refCol+`)
			VALUES ($1, $2, $3, $4, $5) RETURNING `+ticketCols, a.UserID, in.Category, in.Subject, priority, refVal))
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO support_messages (ticket_id, author_id, author_role, body) VALUES ($1, $2, 'user', $3)`, t.ID, a.UserID, in.Body); err != nil {
			return err
		}
		out = t
		return events.Emit(ctx, tx, "support_ticket", t.ID.String(), "SupportTicketCreated", map[string]any{
			"ticket_id": t.ID, "user_id": a.UserID, "category": in.Category, "priority": priority})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) Mine(ctx context.Context, a principal.Actor) ([]Ticket, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+ticketCols+` FROM support_tickets WHERE user_id = $1 ORDER BY last_message_at DESC LIMIT 50`, a.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		t.AssignedTo = nil // klienti nuk sheh kush e ka marrë
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (s *Service) messages(ctx context.Context, ticketID uuid.UUID) ([]Message, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, author_role, body, created_at FROM support_messages WHERE ticket_id = $1 ORDER BY created_at`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.AuthorRole, &m.Body, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Get — tiketa e vet me mesazhet.
func (s *Service) Get(ctx context.Context, a principal.Actor, id uuid.UUID) (*Ticket, error) {
	t, err := scanTicket(s.pool.QueryRow(ctx, `SELECT `+ticketCols+` FROM support_tickets WHERE id = $1 AND user_id = $2`, id, a.UserID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.AssignedTo = nil
	if t.Messages, err = s.messages(ctx, id); err != nil {
		return nil, err
	}
	return t, nil
}

// Reply — mesazh nga përdoruesi (tiketa rikthehet 'open' nëse priste përgjigjen e tij).
func (s *Service) Reply(ctx context.Context, a principal.Actor, id uuid.UUID, body string) (*Message, error) {
	body = normText(body, 2000)
	if utf8.RuneCountInString(body) < 1 {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"body": "required"})
	}
	var m Message
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `SELECT status FROM support_tickets WHERE id = $1 AND user_id = $2 FOR UPDATE`, id, a.UserID).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == "closed" {
			return ErrTicketClosed
		}
		if err := tx.QueryRow(ctx, `INSERT INTO support_messages (ticket_id, author_id, author_role, body) VALUES ($1, $2, 'user', $3)
			RETURNING id, author_role, body, created_at`, id, a.UserID, body).Scan(&m.ID, &m.AuthorRole, &m.Body, &m.CreatedAt); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE support_tickets SET status = CASE WHEN status IN ('pending_user','resolved') THEN 'open' ELSE status END,
			last_message_at = now(), updated_at = now(), resolved_at = NULL WHERE id = $1`, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Close — përdoruesi e mbyll vetë tiketën.
func (s *Service) Close(ctx context.Context, a principal.Actor, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `UPDATE support_tickets SET status = 'closed', updated_at = now() WHERE id = $1 AND user_id = $2 AND status <> 'closed'`, id, a.UserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

// --- siguria ---------------------------------------------------------------------------

type SafetyInput struct {
	Kind        string     `json:"kind"`
	RideID      *uuid.UUID `json:"ride_id"`
	Lat         float64    `json:"lat"`
	Lng         float64    `json:"lng"`
	Description string     `json:"description"`
}

type SafetyReport struct {
	ID        uuid.UUID `json:"id"`
	TicketID  uuid.UUID `json:"ticket_id"`
	Kind      string    `json:"kind"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// ReportSafety — SOS / raport sigurie: tiketë urgjente + ngjarje (Operacionet njoftohen, kanali live i ops).
func (s *Service) ReportSafety(ctx context.Context, a principal.Actor, in SafetyInput) (*SafetyReport, error) {
	in.Kind = strings.ToLower(strings.TrimSpace(in.Kind))
	if !contains(SafetyKinds, in.Kind) {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"kind": "invalid"})
	}
	in.Description = normText(in.Description, 1000)
	var lat, lng *float64
	if p := (geo.Point{Lat: in.Lat, Lng: in.Lng}); p.Valid() {
		lat, lng = &in.Lat, &in.Lng
	}
	if in.RideID != nil {
		var ok bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM rides WHERE id = $1 AND (customer_id = $2 OR driver_id = $2))`, *in.RideID, a.UserID).Scan(&ok); err != nil {
			return nil, err
		}
		if !ok {
			in.RideID = nil
		}
	}
	var out SafetyReport
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		subject := "Raport sigurie: " + in.Kind
		if in.Kind == "sos" {
			subject = "SOS"
		}
		var ticketID uuid.UUID
		if err := tx.QueryRow(ctx, `INSERT INTO support_tickets (user_id, category, subject, priority, ride_id) VALUES ($1, 'safety', $2, 'urgent', $3) RETURNING id`,
			a.UserID, subject, in.RideID).Scan(&ticketID); err != nil {
			return err
		}
		body := in.Description
		if body == "" {
			body = "(pa përshkrim)"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO support_messages (ticket_id, author_id, author_role, body) VALUES ($1, $2, 'user', $3)`, ticketID, a.UserID, body); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO safety_reports (reporter_id, ride_id, ticket_id, kind, lat, lng, description)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, ticket_id, kind, status, created_at`,
			a.UserID, in.RideID, ticketID, in.Kind, lat, lng, nullable(in.Description)).Scan(&out.ID, &out.TicketID, &out.Kind, &out.Status, &out.CreatedAt); err != nil {
			return err
		}
		if err := audit(ctx, tx, a, "safety.reported", "safety_report", out.ID.String(), map[string]any{"kind": in.Kind, "ride_id": in.RideID}); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "safety_report", out.ID.String(), "SafetyReportCreated", map[string]any{
			"report_id": out.ID, "ticket_id": ticketID, "reporter_id": a.UserID, "ride_id": in.RideID, "kind": in.Kind, "lat": lat, "lng": lng})
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// --- Support (agjentët) --------------------------------------------------------------

type QueueFilter struct {
	Status     string
	Priority   string
	AssignedTo *uuid.UUID
	Limit      int
}

func (s *Service) Queue(ctx context.Context, f QueueFilter) ([]Ticket, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if f.Status == "" {
		f.Status = "open"
	}
	rows, err := s.pool.Query(ctx, `SELECT `+ticketCols+` FROM support_tickets
		WHERE ($1 = 'all' OR status = $1) AND ($2 = '' OR priority = $2) AND ($3::uuid IS NULL OR assigned_to = $3)
		ORDER BY CASE priority WHEN 'urgent' THEN 0 WHEN 'high' THEN 1 ELSE 2 END, last_message_at LIMIT $4`, f.Status, f.Priority, f.AssignedTo, f.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

// AgentGet — tiketa me mesazhet dhe kontekstin (audit: qasje në të dhëna personale).
func (s *Service) AgentGet(ctx context.Context, agent principal.Actor, id uuid.UUID) (*Ticket, error) {
	t, err := scanTicket(s.pool.QueryRow(ctx, `SELECT `+ticketCols+` FROM support_tickets WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if t.Messages, err = s.messages(ctx, id); err != nil {
		return nil, err
	}
	c := &Context{}
	if err := s.pool.QueryRow(ctx, `SELECT phone_e164, full_name, locale FROM users WHERE id = $1`, t.UserID).Scan(&c.UserPhone, &c.UserName, &c.UserLocale); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if t.RideID != nil {
		var r RideSummary
		err := s.pool.QueryRow(ctx, `SELECT id, state, category_id, price_quoted_minor, price_final_minor, payment_method, payment_status, driver_id, requested_at, completed_at FROM rides WHERE id = $1`, *t.RideID).
			Scan(&r.ID, &r.State, &r.CategoryID, &r.PriceQuotedMinor, &r.PriceFinalMinor, &r.PaymentMethod, &r.PaymentStatus, &r.DriverID, &r.RequestedAt, &r.CompletedAt)
		if err == nil {
			c.Ride = &r
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}
	t.Context = c
	_, _ = s.pool.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, metadata) VALUES ($1, 'support.ticket_viewed', 'support_ticket', $2, jsonb_build_object('user_id', $3::text))`,
		agent.UserID, id.String(), t.UserID.String())
	return t, nil
}

// AgentReply — përgjigje nga Support-i: tiketa kalon 'pending_user', përdoruesi njoftohet (ngjarje).
func (s *Service) AgentReply(ctx context.Context, agent principal.Actor, id uuid.UUID, body string) (*Message, error) {
	body = normText(body, 2000)
	if utf8.RuneCountInString(body) < 1 {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"body": "required"})
	}
	var m Message
	var userID uuid.UUID
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx, `SELECT status, user_id FROM support_tickets WHERE id = $1 FOR UPDATE`, id).Scan(&status, &userID)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == "closed" {
			return ErrTicketClosed
		}
		if err := tx.QueryRow(ctx, `INSERT INTO support_messages (ticket_id, author_id, author_role, body) VALUES ($1, $2, 'support', $3)
			RETURNING id, author_role, body, created_at`, id, agent.UserID, body).Scan(&m.ID, &m.AuthorRole, &m.Body, &m.CreatedAt); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE support_tickets SET status = 'pending_user', assigned_to = COALESCE(assigned_to, $2), last_message_at = now(), updated_at = now() WHERE id = $1`, id, agent.UserID); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "support_ticket", id.String(), "SupportTicketReplied", map[string]any{"ticket_id": id, "user_id": userID, "message_id": m.ID})
	})
	if err != nil {
		return nil, err
	}
	return &m, nil
}

type AgentUpdate struct {
	Status     *string    `json:"status"`
	Priority   *string    `json:"priority"`
	AssignedTo *uuid.UUID `json:"assigned_to"`
}

func (s *Service) AgentUpdate(ctx context.Context, agent principal.Actor, id uuid.UUID, in AgentUpdate) (*Ticket, error) {
	fields := map[string]string{}
	if in.Status != nil && !contains([]string{"open", "pending_user", "resolved", "closed"}, *in.Status) {
		fields["status"] = "invalid"
	}
	if in.Priority != nil && !contains([]string{"normal", "high", "urgent"}, *in.Priority) {
		fields["priority"] = "invalid"
	}
	if in.Status == nil && in.Priority == nil && in.AssignedTo == nil {
		fields["body"] = "empty"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	var out *Ticket
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		t, err := scanTicket(tx.QueryRow(ctx, `
			UPDATE support_tickets SET status = COALESCE($2, status), priority = COALESCE($3, priority), assigned_to = COALESCE($4, assigned_to),
			  resolved_at = CASE WHEN $2 = 'resolved' THEN now() ELSE resolved_at END, updated_at = now()
			WHERE id = $1 RETURNING `+ticketCols, id, in.Status, in.Priority, in.AssignedTo))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		out = t
		if in.Status != nil && (*in.Status == "resolved" || *in.Status == "closed") {
			if _, err := tx.Exec(ctx, `UPDATE safety_reports SET status = 'closed', updated_at = now() WHERE ticket_id = $1`, id); err != nil {
				return err
			}
		}
		return audit(ctx, tx, agent, "support.ticket_updated", "support_ticket", id.String(), map[string]any{"status": in.Status, "priority": in.Priority, "assigned_to": in.AssignedTo})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func audit(ctx context.Context, tx events.Execer, a principal.Actor, action, targetType, targetID string, meta map[string]any) error {
	var ip *net.IP
	if p := net.ParseIP(a.IP); p != nil {
		ip = &p
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	var reqID *string
	if v := logx.RequestID(ctx); v != "" {
		reqID = &v
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_log (actor_id, action, target_type, target_id, ip, request_id, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, a.UserID, action, targetType, targetID, ip, reqID, metaJSON)
	return err
}
