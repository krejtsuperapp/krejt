// Package reviews — vlerësimet dyanëshe të udhëtimit (§30): klienti vlerëson shoferin, shoferi klientin;
// vetëm pas përfundimit, brenda 7 ditësh, një herë për person; raportim → moderim (Support).
// Agregatet (shuma/numërimi) mbahen te drivers/users dhe përditësohen në të njëjtin transaksion.
package reviews

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

var (
	ErrRideNotReviewable = &httpx.APIError{Code: "RIDE_NOT_REVIEWABLE", MessageKey: "errors.reviews.not_reviewable", HTTPStatus: http.StatusConflict}
	ErrAlreadyReviewed   = &httpx.APIError{Code: "ALREADY_REVIEWED", MessageKey: "errors.reviews.already_reviewed", HTTPStatus: http.StatusConflict}
)

// ReviewWindow — sa kohë pas përfundimit lejohet vlerësimi.
const ReviewWindow = 7 * 24 * time.Hour

// Etiketat e lejuara (klienti i sheh si çipa; përkthimi në klient).
var CustomerTags = []string{"clean_car", "friendly", "safe_driving", "great_route", "late", "rude", "unsafe_driving", "wrong_route", "dirty_car"}
var DriverTags = []string{"polite", "on_time", "clear_pickup", "late", "rude", "no_show", "damaged_car"}

type Input struct {
	Rating  int      `json:"rating"`
	Tags    []string `json:"tags"`
	Comment string   `json:"comment"`
}

type Review struct {
	ID           uuid.UUID `json:"id"`
	RideID       uuid.UUID `json:"ride_id"`
	ReviewerRole string    `json:"reviewer_role"`
	Rating       int       `json:"rating"`
	Tags         []string  `json:"tags"`
	Comment      *string   `json:"comment"`
	CreatedAt    time.Time `json:"created_at"`
}

type Service struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func validate(in *Input, allowed []string) map[string]string {
	f := map[string]string{}
	if in.Rating < 1 || in.Rating > 5 {
		f["rating"] = "invalid"
	}
	seen := map[string]bool{}
	tags := make([]string, 0, len(in.Tags))
	for _, t := range in.Tags {
		t = strings.ToLower(strings.TrimSpace(t))
		ok := false
		for _, a := range allowed {
			if a == t {
				ok = true
			}
		}
		if !ok {
			f["tags"] = "invalid"
			continue
		}
		if !seen[t] {
			seen[t] = true
			tags = append(tags, t)
		}
	}
	if len(tags) > 5 {
		f["tags"] = "too_many"
	}
	in.Tags = tags
	in.Comment = strings.Join(strings.Fields(in.Comment), " ")
	if utf8.RuneCountInString(in.Comment) > 300 {
		f["comment"] = "too_long"
	}
	return f
}

// Create — aktori vlerëson palën tjetër të udhëtimit (rolin e nxjerr serveri, jo klienti).
func (s *Service) Create(ctx context.Context, a principal.Actor, rideID uuid.UUID, in Input) (*Review, error) {
	var out *Review
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var customerID uuid.UUID
		var driverID *uuid.UUID
		var state string
		var completedAt *time.Time
		err := tx.QueryRow(ctx, `SELECT customer_id, driver_id, state, completed_at FROM rides WHERE id = $1 AND (customer_id = $2 OR driver_id = $2) FOR UPDATE`, rideID, a.UserID).
			Scan(&customerID, &driverID, &state, &completedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		if state != "completed" || driverID == nil || completedAt == nil || time.Since(*completedAt) > ReviewWindow {
			return ErrRideNotReviewable
		}
		role, reviewee, allowed := "customer", *driverID, CustomerTags
		if a.UserID == *driverID {
			role, reviewee, allowed = "driver", customerID, DriverTags
		}
		if f := validate(&in, allowed); len(f) > 0 {
			return httpx.ErrValidation.WithFields(f)
		}
		var comment *string
		if in.Comment != "" {
			comment = &in.Comment
		}
		var r Review
		err = tx.QueryRow(ctx, `
			INSERT INTO ride_reviews (ride_id, reviewer_id, reviewee_id, reviewer_role, rating, tags, comment)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id, ride_id, reviewer_role, rating, tags, comment, created_at`,
			rideID, a.UserID, reviewee, role, in.Rating, in.Tags, comment).
			Scan(&r.ID, &r.RideID, &r.ReviewerRole, &r.Rating, &r.Tags, &r.Comment, &r.CreatedAt)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrAlreadyReviewed
			}
			return err
		}
		out = &r
		// agregati i palës së vlerësuar, në të njëjtin transaksion
		table := "users"
		if role == "customer" {
			table = "drivers"
		}
		col := "id"
		if table == "drivers" {
			col = "user_id"
		}
		if _, err := tx.Exec(ctx, `UPDATE `+table+` SET rating_sum = rating_sum + $2, rating_count = rating_count + 1, updated_at = now() WHERE `+col+` = $1`, reviewee, in.Rating); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "ride", rideID.String(), "RideReviewed", map[string]any{
			"ride_id": rideID, "reviewer_id": a.UserID, "reviewee_id": reviewee, "role": role, "rating": in.Rating, "tags": in.Tags})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Report — pala e vlerësuar raporton vlerësimin (abuzim, gjuhë fyese) → flagged për Support.
func (s *Service) Report(ctx context.Context, a principal.Actor, reviewID uuid.UUID, reason string) error {
	reason = strings.Join(strings.Fields(reason), " ")
	if reason == "" || utf8.RuneCountInString(reason) > 300 {
		return httpx.ErrValidation.WithFields(map[string]string{"reason": "required"})
	}
	tag, err := s.pool.Exec(ctx, `UPDATE ride_reviews SET moderation_status = 'flagged', report_reason = $3, reported_at = now()
		WHERE id = $1 AND reviewee_id = $2 AND moderation_status = 'visible'`, reviewID, a.UserID, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	return nil
}

// ForRide — vlerësimet e udhëtimit që i sheh aktori: të vetat gjithmonë; të palës tjetër vetëm nëse janë visible.
func (s *Service) ForRide(ctx context.Context, a principal.Actor, rideID uuid.UUID) ([]Review, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT r.id, r.ride_id, r.reviewer_role, r.rating, r.tags, r.comment, r.created_at
		FROM ride_reviews r JOIN rides x ON x.id = r.ride_id
		WHERE r.ride_id = $1 AND (x.customer_id = $2 OR x.driver_id = $2)
		  AND (r.reviewer_id = $2 OR r.moderation_status = 'visible')
		ORDER BY r.created_at`, rideID, a.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Review{}
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.ID, &r.RideID, &r.ReviewerRole, &r.Rating, &r.Tags, &r.Comment, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// --- HTTP ------------------------------------------------------------------------

func (s *Service) Routes(mux *http.ServeMux, requireAuth httpx.Middleware) {
	mux.Handle("POST /api/v1/rides/{id}/review", requireAuth(principal.Handler(s.handleCreate)))
	mux.Handle("GET /api/v1/rides/{id}/reviews", requireAuth(principal.Handler(s.handleForRide)))
	mux.Handle("POST /api/v1/reviews/{id}/report", requireAuth(principal.Handler(s.handleReport)))

	// Porositë: klienti vlerëson lokalin pas dorëzimit.
	mux.Handle("POST /api/v1/orders/{id}/review", requireAuth(principal.Handler(s.handleCreateForOrder)))
	mux.Handle("GET /api/v1/orders/{id}/review", requireAuth(principal.Handler(s.handleMyOrderReview)))
}

func (s *Service) handleCreateForOrder(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in Input
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rev, err := s.CreateForOrder(r.Context(), a, id, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, rev)
}

// handleMyOrderReview — 204 kur nuk ka vlerësim: ekrani e pyet para se ta ofrojë, dhe "nuk ka"
// nuk është gabim.
func (s *Service) handleMyOrderReview(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rev, err := s.MyOrderReview(r.Context(), a, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if rev == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, rev)
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		return uuid.Nil, httpx.ErrNotFound
	}
	return id, nil
}

func (s *Service) handleCreate(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in Input
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	rev, err := s.Create(r.Context(), a, id, in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, rev)
}

func (s *Service) handleForRide(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items, err := s.ForRide(r.Context(), a, id)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items, "customer_tags": CustomerTags, "driver_tags": DriverTags})
}

func (s *Service) handleReport(w http.ResponseWriter, r *http.Request, a principal.Actor) {
	id, err := pathID(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err := httpx.DecodeJSON(r, &in); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if err := s.Report(r.Context(), a, id, in.Reason); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
