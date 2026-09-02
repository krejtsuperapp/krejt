// Package catalog — menuja/katalogu i merchant-it (§19, §21): kategori, produkte, modifikues me rregulla
// min/max, disponueshmëri, menu publike e plotë (një thirrje) me cache të shkurtër. Çmimet vetëm në cent.
// Vlerësimi i një zgjedhjeje (produkt + opsione) bëhet këtu në server (orders e thërret) — kurrë nga klienti.
package catalog

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/principal"
)

var (
	ErrUnavailable   = &httpx.APIError{Code: "PRODUCT_UNAVAILABLE", MessageKey: "errors.catalog.unavailable", HTTPStatus: http.StatusConflict}
	ErrModifiers     = &httpx.APIError{Code: "MODIFIERS_INVALID", MessageKey: "errors.catalog.modifiers_invalid", HTTPStatus: http.StatusUnprocessableEntity}
	ErrWrongMerchant = &httpx.APIError{Code: "PRODUCT_WRONG_MERCHANT", MessageKey: "errors.catalog.wrong_merchant", HTTPStatus: http.StatusUnprocessableEntity}
)

var Units = []string{"piece", "kg", "g", "l", "ml", "portion"}

type Category struct {
	ID     uuid.UUID `json:"id"`
	Name   string    `json:"name"`
	Sort   int       `json:"sort"`
	Active bool      `json:"active"`
}

type Option struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	PriceDeltaMinor int64     `json:"price_delta_minor"`
	Available       bool      `json:"available"`
	Sort            int       `json:"sort"`
}

type ModifierGroup struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	MinSelect int       `json:"min_select"`
	MaxSelect int       `json:"max_select"`
	Sort      int       `json:"sort"`
	Options   []Option  `json:"options"`
}

type Product struct {
	ID          uuid.UUID       `json:"id"`
	MerchantID  uuid.UUID       `json:"merchant_id"`
	CategoryID  *uuid.UUID      `json:"category_id"`
	Name        string          `json:"name"`
	Description *string         `json:"description"`
	PriceMinor  int64           `json:"price_minor"`
	Currency    string          `json:"currency"`
	ImageKey    *string         `json:"image_key"`
	Available   bool            `json:"available"`
	Unit        string          `json:"unit"`
	Tags        []string        `json:"tags"`
	Sort        int             `json:"sort"`
	Modifiers   []ModifierGroup `json:"modifiers"`
}

type Menu struct {
	MerchantID  uuid.UUID  `json:"merchant_id"`
	Categories  []Category `json:"categories"`
	Products    []Product  `json:"products"`
	GeneratedAt time.Time  `json:"generated_at"`
}

// Membership — verifikimi i stafit (nga moduli merchants) — injektohet për të shmangur varësi ciklike.
type Membership interface {
	Membership(ctx context.Context, userID, merchantID uuid.UUID) (string, error)
}

type Service struct {
	pool   *pgxpool.Pool
	member Membership
	mu     sync.Mutex
	cache  map[uuid.UUID]cachedMenu
	now    func() time.Time
}

type cachedMenu struct {
	menu *Menu
	at   time.Time
}

const MenuCacheTTL = 30 * time.Second

func New(pool *pgxpool.Pool, m Membership) *Service {
	return &Service{pool: pool, member: m, cache: map[uuid.UUID]cachedMenu{}, now: time.Now}
}

func (s *Service) requireStaff(ctx context.Context, a principal.Actor, merchantID uuid.UUID) error {
	_, err := s.member.Membership(ctx, a.UserID, merchantID)
	return err
}

func (s *Service) invalidate(merchantID uuid.UUID) {
	s.mu.Lock()
	delete(s.cache, merchantID)
	s.mu.Unlock()
}

// --- kategoritë ------------------------------------------------------------------------------

type CategoryInput struct {
	Name   string `json:"name"`
	Sort   int    `json:"sort"`
	Active *bool  `json:"active"`
}

func (s *Service) UpsertCategory(ctx context.Context, a principal.Actor, merchantID uuid.UUID, id *uuid.UUID, in CategoryInput) (*Category, error) {
	if err := s.requireStaff(ctx, a, merchantID); err != nil {
		return nil, err
	}
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 60 {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"name": "invalid"})
	}
	active := true
	if in.Active != nil {
		active = *in.Active
	}
	var c Category
	var err error
	if id == nil {
		err = s.pool.QueryRow(ctx, `INSERT INTO catalog_categories (merchant_id, name, sort, active) VALUES ($1, $2, $3, $4) RETURNING id, name, sort, active`,
			merchantID, in.Name, in.Sort, active).Scan(&c.ID, &c.Name, &c.Sort, &c.Active)
	} else {
		err = s.pool.QueryRow(ctx, `UPDATE catalog_categories SET name = $3, sort = $4, active = $5 WHERE id = $2 AND merchant_id = $1 RETURNING id, name, sort, active`,
			merchantID, *id, in.Name, in.Sort, active).Scan(&c.ID, &c.Name, &c.Sort, &c.Active)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, httpx.ErrNotFound
		}
	}
	if err != nil {
		return nil, err
	}
	s.invalidate(merchantID)
	return &c, nil
}

func (s *Service) DeleteCategory(ctx context.Context, a principal.Actor, merchantID, id uuid.UUID) error {
	if err := s.requireStaff(ctx, a, merchantID); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM catalog_categories WHERE id = $2 AND merchant_id = $1`, merchantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	s.invalidate(merchantID)
	return nil
}

// --- produktet ----------------------------------------------------------------------------------

type OptionInput struct {
	Name            string `json:"name"`
	PriceDeltaMinor int64  `json:"price_delta_minor"`
	Available       *bool  `json:"available"`
}

type ModifierGroupInput struct {
	Name      string        `json:"name"`
	MinSelect int           `json:"min_select"`
	MaxSelect int           `json:"max_select"`
	Options   []OptionInput `json:"options"`
}

type ProductInput struct {
	CategoryID  *uuid.UUID           `json:"category_id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	PriceMinor  int64                `json:"price_minor"`
	ImageKey    *string              `json:"image_key"`
	Available   *bool                `json:"available"`
	Unit        string               `json:"unit"`
	Tags        []string             `json:"tags"`
	Sort        int                  `json:"sort"`
	Modifiers   []ModifierGroupInput `json:"modifiers"` // zëvendësohen plotësisht kur dërgohen (nil = pa ndryshim)
}

func validateProduct(in *ProductInput) map[string]string {
	f := map[string]string{}
	in.Name = strings.Join(strings.Fields(in.Name), " ")
	if n := utf8.RuneCountInString(in.Name); n < 1 || n > 120 {
		f["name"] = "invalid"
	}
	in.Description = strings.TrimSpace(in.Description)
	if utf8.RuneCountInString(in.Description) > 600 {
		f["description"] = "too_long"
	}
	if in.PriceMinor < 0 || in.PriceMinor > 100000000 {
		f["price_minor"] = "invalid"
	}
	if in.Unit == "" {
		in.Unit = "piece"
	}
	ok := false
	for _, u := range Units {
		if u == in.Unit {
			ok = true
		}
	}
	if !ok {
		f["unit"] = "invalid"
	}
	if len(in.Modifiers) > 10 {
		f["modifiers"] = "too_many"
	}
	for i := range in.Modifiers {
		g := &in.Modifiers[i]
		g.Name = strings.Join(strings.Fields(g.Name), " ")
		if g.Name == "" || utf8.RuneCountInString(g.Name) > 60 {
			f["modifiers"] = "name"
		}
		if g.MaxSelect < 1 {
			g.MaxSelect = 1
		}
		if g.MinSelect < 0 || g.MinSelect > g.MaxSelect || len(g.Options) == 0 || len(g.Options) > 30 {
			f["modifiers"] = "rules"
		}
		for j := range g.Options {
			o := &g.Options[j]
			o.Name = strings.Join(strings.Fields(o.Name), " ")
			if o.Name == "" || utf8.RuneCountInString(o.Name) > 60 || o.PriceDeltaMinor < -100000 || o.PriceDeltaMinor > 100000 {
				f["modifiers"] = "option"
			}
		}
	}
	tags := []string{}
	seen := map[string]bool{}
	for _, t := range in.Tags {
		t = strings.ToLower(strings.TrimSpace(t))
		if t != "" && !seen[t] && len(tags) < 10 {
			seen[t] = true
			tags = append(tags, t)
		}
	}
	in.Tags = tags
	return f
}

const productCols = `id, merchant_id, category_id, name, description, price_minor, currency, image_key, available, unit, tags, sort`

func scanProduct(row pgx.Row) (*Product, error) {
	var p Product
	if err := row.Scan(&p.ID, &p.MerchantID, &p.CategoryID, &p.Name, &p.Description, &p.PriceMinor, &p.Currency, &p.ImageKey, &p.Available, &p.Unit, &p.Tags, &p.Sort); err != nil {
		return nil, err
	}
	p.Currency = strings.TrimSpace(p.Currency)
	p.Modifiers = []ModifierGroup{}
	return &p, nil
}

func (s *Service) UpsertProduct(ctx context.Context, a principal.Actor, merchantID uuid.UUID, id *uuid.UUID, in ProductInput) (*Product, error) {
	if err := s.requireStaff(ctx, a, merchantID); err != nil {
		return nil, err
	}
	if f := validateProduct(&in); len(f) > 0 {
		return nil, httpx.ErrValidation.WithFields(f)
	}
	if in.CategoryID != nil {
		var ok bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM catalog_categories WHERE id = $1 AND merchant_id = $2)`, *in.CategoryID, merchantID).Scan(&ok); err != nil {
			return nil, err
		}
		if !ok {
			return nil, httpx.ErrValidation.WithFields(map[string]string{"category_id": "invalid"})
		}
	}
	available := true
	if in.Available != nil {
		available = *in.Available
	}
	var out *Product
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var p *Product
		var err error
		if id == nil {
			p, err = scanProduct(tx.QueryRow(ctx, `INSERT INTO products (merchant_id, category_id, name, description, price_minor, image_key, available, unit, tags, sort)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING `+productCols,
				merchantID, in.CategoryID, in.Name, nullable(in.Description), in.PriceMinor, in.ImageKey, available, in.Unit, in.Tags, in.Sort))
		} else {
			p, err = scanProduct(tx.QueryRow(ctx, `UPDATE products SET category_id = $3, name = $4, description = $5, price_minor = $6, image_key = COALESCE($7, image_key),
				available = $8, unit = $9, tags = $10, sort = $11, updated_at = now() WHERE id = $2 AND merchant_id = $1 AND deleted_at IS NULL RETURNING `+productCols,
				merchantID, *id, in.CategoryID, in.Name, nullable(in.Description), in.PriceMinor, in.ImageKey, available, in.Unit, in.Tags, in.Sort))
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.ErrNotFound
			}
		}
		if err != nil {
			return err
		}
		if in.Modifiers != nil {
			if _, err := tx.Exec(ctx, `DELETE FROM modifier_groups WHERE product_id = $1`, p.ID); err != nil {
				return err
			}
			for gi, g := range in.Modifiers {
				var gid uuid.UUID
				if err := tx.QueryRow(ctx, `INSERT INTO modifier_groups (product_id, name, min_select, max_select, sort) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
					p.ID, g.Name, g.MinSelect, g.MaxSelect, gi).Scan(&gid); err != nil {
					return err
				}
				for oi, o := range g.Options {
					av := true
					if o.Available != nil {
						av = *o.Available
					}
					if _, err := tx.Exec(ctx, `INSERT INTO modifier_options (group_id, name, price_delta_minor, available, sort) VALUES ($1, $2, $3, $4, $5)`, gid, o.Name, o.PriceDeltaMinor, av, oi); err != nil {
						return err
					}
				}
			}
		}
		out = p
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.invalidate(merchantID)
	if err := s.loadModifiers(ctx, []*Product{out}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) SetAvailability(ctx context.Context, a principal.Actor, merchantID, productID uuid.UUID, available bool) error {
	if err := s.requireStaff(ctx, a, merchantID); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE products SET available = $3, updated_at = now() WHERE id = $2 AND merchant_id = $1 AND deleted_at IS NULL`, merchantID, productID, available)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	s.invalidate(merchantID)
	return nil
}

func (s *Service) DeleteProduct(ctx context.Context, a principal.Actor, merchantID, productID uuid.UUID) error {
	if err := s.requireStaff(ctx, a, merchantID); err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE products SET deleted_at = now(), available = false, updated_at = now() WHERE id = $2 AND merchant_id = $1 AND deleted_at IS NULL`, merchantID, productID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return httpx.ErrNotFound
	}
	s.invalidate(merchantID)
	return nil
}

func (s *Service) loadModifiers(ctx context.Context, products []*Product) error {
	if len(products) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(products))
	byID := map[uuid.UUID]*Product{}
	for _, p := range products {
		ids = append(ids, p.ID)
		byID[p.ID] = p
	}
	rows, err := s.pool.Query(ctx, `
		SELECT g.product_id, g.id, g.name, g.min_select, g.max_select, g.sort, o.id, o.name, o.price_delta_minor, o.available, o.sort
		FROM modifier_groups g LEFT JOIN modifier_options o ON o.group_id = g.id
		WHERE g.product_id = ANY($1) ORDER BY g.product_id, g.sort, g.id, o.sort`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	groups := map[uuid.UUID]*ModifierGroup{}
	for rows.Next() {
		var pid, gid uuid.UUID
		var g ModifierGroup
		var oid *uuid.UUID
		var oname *string
		var odelta *int64
		var oavail *bool
		var osort *int
		if err := rows.Scan(&pid, &gid, &g.Name, &g.MinSelect, &g.MaxSelect, &g.Sort, &oid, &oname, &odelta, &oavail, &osort); err != nil {
			return err
		}
		grp, ok := groups[gid]
		if !ok {
			g.ID = gid
			g.Options = []Option{}
			p := byID[pid]
			p.Modifiers = append(p.Modifiers, g)
			grp = &p.Modifiers[len(p.Modifiers)-1]
			groups[gid] = grp
		}
		if oid != nil {
			grp.Options = append(grp.Options, Option{ID: *oid, Name: *oname, PriceDeltaMinor: *odelta, Available: *oavail, Sort: *osort})
		}
	}
	return rows.Err()
}

// Menu — menuja e plotë (cache 30 s); includeUnavailable për stafin.
func (s *Service) Menu(ctx context.Context, merchantID uuid.UUID, includeUnavailable bool) (*Menu, error) {
	if !includeUnavailable {
		s.mu.Lock()
		if c, ok := s.cache[merchantID]; ok && s.now().Sub(c.at) < MenuCacheTTL {
			s.mu.Unlock()
			return c.menu, nil
		}
		s.mu.Unlock()
	}
	m := &Menu{MerchantID: merchantID, Categories: []Category{}, Products: []Product{}, GeneratedAt: s.now()}
	rows, err := s.pool.Query(ctx, `SELECT id, name, sort, active FROM catalog_categories WHERE merchant_id = $1 AND (active OR $2) ORDER BY sort, name`, merchantID, includeUnavailable)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Sort, &c.Active); err != nil {
			rows.Close()
			return nil, err
		}
		m.Categories = append(m.Categories, c)
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT `+productCols+` FROM products WHERE merchant_id = $1 AND deleted_at IS NULL AND (available OR $2) ORDER BY category_id, sort, name`, merchantID, includeUnavailable)
	if err != nil {
		return nil, err
	}
	var ptrs []*Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		ptrs = append(ptrs, p)
	}
	rows.Close()
	if err := s.loadModifiers(ctx, ptrs); err != nil {
		return nil, err
	}
	for _, p := range ptrs {
		m.Products = append(m.Products, *p)
	}
	if !includeUnavailable {
		s.mu.Lock()
		s.cache[merchantID] = cachedMenu{menu: m, at: s.now()}
		s.mu.Unlock()
	}
	return m, nil
}

// --- vlerësimi server-side i një zgjedhjeje (për orders) ------------------------------------------

type Selection struct {
	ProductID uuid.UUID   `json:"product_id"`
	OptionIDs []uuid.UUID `json:"option_ids"`
	Quantity  int         `json:"quantity"`
}

type PricedLine struct {
	ProductID  uuid.UUID   `json:"product_id"`
	Name       string      `json:"name"`
	Options    []string    `json:"options"`
	OptionIDs  []uuid.UUID `json:"option_ids"`
	UnitMinor  int64       `json:"unit_minor"` // produkt + opsione
	Quantity   int         `json:"quantity"`
	TotalMinor int64       `json:"total_minor"`
	Currency   string      `json:"currency"`
}

// Price — çmimi i një zgjedhjeje nga baza: produkt aktiv i merchant-it, opsionet brenda grupeve të tij,
// rregullat min/max të çdo grupi të respektuara. Klienti dërgon vetëm id dhe sasi.
func (s *Service) Price(ctx context.Context, merchantID uuid.UUID, sel Selection) (*PricedLine, error) {
	if sel.Quantity < 1 || sel.Quantity > 50 {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"quantity": "invalid"})
	}
	p, err := scanProduct(s.pool.QueryRow(ctx, `SELECT `+productCols+` FROM products WHERE id = $1 AND deleted_at IS NULL`, sel.ProductID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if p.MerchantID != merchantID {
		return nil, ErrWrongMerchant
	}
	if !p.Available {
		return nil, ErrUnavailable
	}
	if err := s.loadModifiers(ctx, []*Product{p}); err != nil {
		return nil, err
	}
	chosen := map[uuid.UUID]bool{}
	for _, id := range sel.OptionIDs {
		chosen[id] = true
	}
	line := &PricedLine{ProductID: p.ID, Name: p.Name, Options: []string{}, OptionIDs: []uuid.UUID{}, UnitMinor: p.PriceMinor, Quantity: sel.Quantity, Currency: p.Currency}
	matched := 0
	for _, g := range p.Modifiers {
		count := 0
		for _, o := range g.Options {
			if chosen[o.ID] {
				if !o.Available {
					return nil, ErrUnavailable
				}
				count++
				matched++
				line.UnitMinor += o.PriceDeltaMinor
				line.Options = append(line.Options, g.Name+": "+o.Name)
				line.OptionIDs = append(line.OptionIDs, o.ID)
			}
		}
		if count < g.MinSelect || count > g.MaxSelect {
			return nil, ErrModifiers
		}
	}
	if matched != len(chosen) {
		return nil, ErrModifiers // opsion i një produkti tjetër
	}
	if line.UnitMinor < 0 {
		line.UnitMinor = 0
	}
	line.TotalMinor = line.UnitMinor * int64(sel.Quantity)
	return line, nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
