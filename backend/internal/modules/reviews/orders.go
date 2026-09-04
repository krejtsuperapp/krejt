package reviews

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"krejt.app/backend/internal/platform/events"
	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

// Vlerësimi i një porosie (§30).
//
// Deri tani `merchants.rating_sum` nuk e shkruante asnjë rresht kodi, ndërsa lista e Ushqimit e
// kishte vendin gati për yllin. Një notë që askush nuk mund ta japë nuk duhet të shfaqet fare;
// prandaj kjo pjesë erdhi para se ylli të fshihej.

// MerchantTags — etiketat e lejuara për një lokal. Të ndara nga ato të udhëtimit: «rrugë e mirë»
// nuk do të thotë gjë për një kuzhinë.
var MerchantTags = []string{
	"tasty", "hot", "well_packed", "fast", "accurate",
	"cold", "late", "wrong_items", "small_portion",
}

var ErrOrderNotReviewable = &httpx.APIError{
	Code:       "ORDER_NOT_REVIEWABLE",
	MessageKey: "errors.reviews.order_not_reviewable",
	HTTPStatus: http.StatusConflict,
}

type OrderReview struct {
	ID        uuid.UUID `json:"id"`
	OrderID   uuid.UUID `json:"order_id"`
	Rating    int       `json:"rating"`
	Tags      []string  `json:"tags"`
	Comment   *string   `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateForOrder — vetëm klienti i porosisë, vetëm pas dorëzimit, vetëm brenda dritares.
// Agregati i lokalit përditësohet në të njëjtin transaksion: një notë e shkruar dhe një mesatare
// e pandryshuar do të ishin dy të vërteta të ndryshme për të njëjtën gjë.
func (s *Service) CreateForOrder(ctx context.Context, a principal.Actor, orderID uuid.UUID, in Input) (*OrderReview, error) {
	var out *OrderReview
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var customerID, merchantID uuid.UUID
		var state string
		var deliveredAt *time.Time
		err := tx.QueryRow(ctx,
			`SELECT customer_id, merchant_id, state, delivered_at FROM orders WHERE id = $1 FOR UPDATE`, orderID).
			Scan(&customerID, &merchantID, &state, &deliveredAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		// Porosia e dikujt tjetër nuk ekziston për këtë përdorues.
		if customerID != a.UserID {
			return httpx.ErrNotFound
		}
		if state != "delivered" || deliveredAt == nil || time.Since(*deliveredAt) > ReviewWindow {
			return ErrOrderNotReviewable
		}
		if f := validate(&in, MerchantTags); len(f) > 0 {
			return httpx.ErrValidation.WithFields(f)
		}
		var comment *string
		if in.Comment != "" {
			comment = &in.Comment
		}
		var r OrderReview
		err = tx.QueryRow(ctx, `
			INSERT INTO order_reviews (order_id, reviewer_id, merchant_id, rating, tags, comment)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, order_id, rating, tags, comment, created_at`,
			orderID, a.UserID, merchantID, in.Rating, in.Tags, comment).
			Scan(&r.ID, &r.OrderID, &r.Rating, &r.Tags, &r.Comment, &r.CreatedAt)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return ErrAlreadyReviewed
			}
			return err
		}
		out = &r
		if _, err := tx.Exec(ctx,
			`UPDATE merchants SET rating_sum = rating_sum + $2, rating_count = rating_count + 1, updated_at = now() WHERE id = $1`,
			merchantID, in.Rating); err != nil {
			return err
		}
		return events.Emit(ctx, tx, "order", orderID.String(), "OrderReviewed", map[string]any{
			"order_id": orderID, "merchant_id": merchantID, "reviewer_id": a.UserID,
			"rating": in.Rating, "tags": in.Tags,
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MyOrderReview — vlerësimi im për këtë porosi, ose null. Ekrani e përdor që të mos e ofrojë dy
// herë të njëjtën gjë.
func (s *Service) MyOrderReview(ctx context.Context, a principal.Actor, orderID uuid.UUID) (*OrderReview, error) {
	var r OrderReview
	err := s.pool.QueryRow(ctx, `
		SELECT id, order_id, rating, tags, comment, created_at
		FROM order_reviews WHERE order_id = $1 AND reviewer_id = $2`, orderID, a.UserID).
		Scan(&r.ID, &r.OrderID, &r.Rating, &r.Tags, &r.Comment, &r.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}
