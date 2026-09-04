package services

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"krejt.app/backend/internal/domain/geo"
	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// OpenRequest — kërkesa e hapur siç e sheh mjeshtri: pa emrin dhe pa telefonin e klientit derisa
// oferta të pranohet, dhe pa adresën e saktë — vetëm rrethina, që privatësia të mbetet e mbrojtur (§57).
type OpenRequest struct {
	ID          uuid.UUID  `json:"id"`
	Code        string     `json:"code"`
	CategoryID  string     `json:"category_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	City        string     `json:"city"`
	DistanceM   int        `json:"distance_m"`
	PreferredAt *time.Time `json:"preferred_at"`
	PhotoKeys   []string   `json:"photo_keys"`
	CreatedAt   time.Time  `json:"created_at"`
	MyOffer     *Offer     `json:"my_offer,omitempty"`
}

// approved — mjeshtri duhet të jetë i miratuar dhe kategoria të jetë e tija.
func (s *Service) approved(ctx context.Context, id uuid.UUID, category string) error {
	var status string
	var cats []string
	err := s.pool.QueryRow(ctx, `SELECT status, categories FROM service_providers WHERE user_id = $1`, id).Scan(&status, &cats)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotApproved
	}
	if err != nil {
		return err
	}
	if status != "approved" {
		return ErrNotApproved
	}
	if category != "" && !contains(cats, category) {
		return ErrWrongCategory
	}
	return nil
}

// OpenRequests — punët e hapura në kategoritë e mjeshtrit. Distanca matet nga qyteti i tij vetëm
// si orientim; adresa e saktë i jepet pasi klienti ta pranojë ofertën.
func (s *Service) OpenRequests(ctx context.Context, a principal.Actor, limit int) ([]OpenRequest, error) {
	if err := s.approved(ctx, a.UserID, ""); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.code, r.category_id, r.title, r.description, r.address_lat, r.address_lng, r.preferred_at, r.photo_keys, r.created_at,
		       o.id, o.price_minor, o.currency, o.note, o.can_start_at, o.state, o.created_at
		FROM service_requests r
		JOIN service_providers p ON p.user_id = $1
		LEFT JOIN service_offers o ON o.request_id = r.id AND o.provider_id = $1
		WHERE r.state = 'open' AND r.category_id = ANY(p.categories)
		ORDER BY r.created_at DESC LIMIT $2`, a.UserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []OpenRequest{}
	for rows.Next() {
		var r OpenRequest
		var lat, lng float64
		// LEFT JOIN: pa ofertë të kësaj kërkese, të gjitha kolonat e saj vijnë NULL, ndaj lexohen
		// te tipa që e mbajnë mungesën — përndryshe skanimi dështon dhe lista bie e tëra.
		var offerID *uuid.UUID
		var price *int64
		var currency, state *string
		var note *string
		var canStartAt, offerCreatedAt *time.Time
		if err := rows.Scan(&r.ID, &r.Code, &r.CategoryID, &r.Title, &r.Description, &lat, &lng, &r.PreferredAt, &r.PhotoKeys, &r.CreatedAt,
			&offerID, &price, &currency, &note, &canStartAt, &state, &offerCreatedAt); err != nil {
			return nil, err
		}
		r.City = geo.CityOf(geo.Point{Lat: lat, Lng: lng})
		if offerID != nil {
			o := Offer{ID: *offerID, RequestID: r.ID, ProviderID: a.UserID, Note: note, CanStartAt: canStartAt}
			if price != nil {
				o.PriceMinor = *price
			}
			if currency != nil {
				o.Currency = *currency
			}
			if state != nil {
				o.State = *state
			}
			if offerCreatedAt != nil {
				o.CreatedAt = *offerCreatedAt
			}
			r.MyOffer = &o
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type OfferInput struct {
	PriceMinor int64      `json:"price_minor"`
	Note       string     `json:"note"`
	CanStartAt *time.Time `json:"can_start_at"`
}

// MakeOffer — mjeshtri jep çmimin e vet. Një ofertë për kërkesë; përditësimi lejohet derisa
// klienti të zgjedhë.
func (s *Service) MakeOffer(ctx context.Context, a principal.Actor, requestID uuid.UUID, in OfferInput) (*Offer, error) {
	if in.PriceMinor <= 0 || in.PriceMinor > 500000 {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"price_minor": "range"})
	}
	var category, state string
	err := s.pool.QueryRow(ctx, `SELECT category_id, state FROM service_requests WHERE id = $1`, requestID).Scan(&category, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if state != StateOpen {
		return nil, ErrInvalidState
	}
	if err := s.approved(ctx, a.UserID, category); err != nil {
		return nil, err
	}
	var o Offer
	err = s.pool.QueryRow(ctx, `
		INSERT INTO service_offers (request_id, provider_id, price_minor, note, can_start_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (request_id, provider_id) DO UPDATE SET price_minor = EXCLUDED.price_minor, note = EXCLUDED.note,
		  can_start_at = EXCLUDED.can_start_at, state = 'offered', responded_at = NULL
		WHERE service_offers.state IN ('offered','declined')
		RETURNING id, request_id, provider_id, price_minor, currency, note, can_start_at, state, created_at`,
		requestID, a.UserID, in.PriceMinor, nullable(clip(in.Note, 300)), in.CanStartAt).
		Scan(&o.ID, &o.RequestID, &o.ProviderID, &o.PriceMinor, &o.Currency, &o.Note, &o.CanStartAt, &o.State, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAlreadyOffered
	}
	if err != nil {
		return nil, err
	}
	if err := events.Emit(ctx, s.pool, "service_request", requestID.String(), "ServiceOffered", map[string]any{
		"request_id": requestID, "provider_id": a.UserID, "offer_id": o.ID, "price_minor": in.PriceMinor}); err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Service) WithdrawOffer(ctx context.Context, a principal.Actor, offerID uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `UPDATE service_offers SET state = 'withdrawn', responded_at = now()
		WHERE id = $1 AND provider_id = $2 AND state = 'offered'`, offerID, a.UserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

// MyJobs — punët e mjeshtrit (të rezervuara, në ecje, të përfunduara).
func (s *Service) MyJobs(ctx context.Context, a principal.Actor, limit int) ([]Request, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `SELECT `+requestCols+` FROM service_requests WHERE provider_id = $1 ORDER BY created_at DESC LIMIT $2`, a.UserID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Request{}
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// Step — hapat e mjeshtrit: nis punën, përfundoje, ose hiq dorë para nisjes.
func (s *Service) Step(ctx context.Context, a principal.Actor, id uuid.UUID, to, reason string) (*Request, error) {
	var out *Request
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		r, err := scanRequest(tx.QueryRow(ctx, `SELECT `+requestCols+` FROM service_requests WHERE id = $1 AND provider_id = $2 FOR UPDATE`, id, a.UserID))
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if !CanTransition(r.State, to) {
			return ErrInvalidState
		}
		from := r.State
		set := ""
		switch to {
		case StateInProgress:
			set = `state = 'in_progress', started_at = now()`
		case StateCompleted:
			set = `state = 'completed', completed_at = now(), payment_status = 'pending'`
		case StateOpen:
			set = `state = 'open', provider_id = NULL, accepted_offer_id = NULL, price_minor = NULL, commission_minor = 0, booked_at = NULL`
		default:
			return ErrInvalidState
		}
		r, err = scanRequest(tx.QueryRow(ctx, `UPDATE service_requests SET `+set+`, updated_at = now() WHERE id = $1 RETURNING `+requestCols, id))
		if err != nil {
			return err
		}
		out = r
		if to == StateOpen {
			if _, err := tx.Exec(ctx, `UPDATE service_offers SET state = 'declined', responded_at = now() WHERE request_id = $1 AND provider_id = $2`, id, a.UserID); err != nil {
				return err
			}
		}
		if to == StateCompleted {
			if _, err := tx.Exec(ctx, `UPDATE service_providers SET jobs_done = jobs_done + 1, updated_at = now() WHERE user_id = $1`, a.UserID); err != nil {
				return err
			}
		}
		if err := requestEvent(ctx, tx, id, &from, to, "provider", &a.UserID, map[string]any{"reason": reason}); err != nil {
			return err
		}
		evt := map[string]string{StateInProgress: "ServiceStarted", StateCompleted: "ServiceCompleted", StateOpen: "ServiceReleased"}[to]
		return events.Emit(ctx, tx, "service_request", id.String(), evt, map[string]any{
			"request_id": id, "code": r.Code, "customer_id": r.CustomerID, "provider_id": a.UserID, "reason": reason})
	})
	if err != nil {
		return nil, err
	}
	if to == StateCompleted {
		_ = s.settle(ctx, out)
		out, err = scanRequest(s.pool.QueryRow(ctx, `SELECT `+requestCols+` FROM service_requests WHERE id = $1`, id))
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}
