// Package appconfig — konfigurimi që klientët e marrin në nisje (§64 versionet, §65 feature flags):
// GET /config publik me cache 30 s, porta e versionit (update i detyrueshëm, mirëmbajtje) dhe
// vlerësimi i flag-eve në server (rollout për përqindje, determinist për përdorues).
package appconfig

import (
	"context"
	"encoding/json"
	"errors"
	"hash/fnv"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"krejt.app/backend/internal/platform/httpx"
	"krejt.app/backend/internal/platform/logx"
	"krejt.app/backend/internal/platform/principal"
)

const CacheTTL = 30 * time.Second

type AppVersion struct {
	App                string  `json:"app"`
	Platform           string  `json:"platform"`
	MinVersion         string  `json:"min_version"`
	RecommendedVersion string  `json:"recommended_version"`
	Maintenance        bool    `json:"maintenance"`
	MaintenanceMessage *string `json:"maintenance_message"`
}

type Flag struct {
	Key            string  `json:"key"`
	Enabled        bool    `json:"enabled"`
	RolloutPercent int     `json:"rollout_percent"`
	Public         bool    `json:"public"`
	Description    *string `json:"description,omitempty"`
}

type snapshot struct {
	versions map[string]AppVersion // app/platform
	flags    map[string]Flag
	loadedAt time.Time
}

type Service struct {
	pool *pgxpool.Pool
	mu   sync.RWMutex
	snap *snapshot
	now  func() time.Time
}

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool, now: time.Now} }

func (s *Service) load(ctx context.Context) (*snapshot, error) {
	s.mu.RLock()
	if s.snap != nil && s.now().Sub(s.snap.loadedAt) < CacheTTL {
		defer s.mu.RUnlock()
		return s.snap, nil
	}
	s.mu.RUnlock()
	snap := &snapshot{versions: map[string]AppVersion{}, flags: map[string]Flag{}, loadedAt: s.now()}
	rows, err := s.pool.Query(ctx, `SELECT app, platform, min_version, recommended_version, maintenance, maintenance_message FROM app_versions`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var v AppVersion
		if err := rows.Scan(&v.App, &v.Platform, &v.MinVersion, &v.RecommendedVersion, &v.Maintenance, &v.MaintenanceMessage); err != nil {
			rows.Close()
			return nil, err
		}
		snap.versions[v.App+"/"+v.Platform] = v
	}
	rows.Close()
	rows, err = s.pool.Query(ctx, `SELECT key, enabled, rollout_percent, public, description FROM feature_flags`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var f Flag
		if err := rows.Scan(&f.Key, &f.Enabled, &f.RolloutPercent, &f.Public, &f.Description); err != nil {
			rows.Close()
			return nil, err
		}
		snap.flags[f.Key] = f
	}
	rows.Close()
	s.mu.Lock()
	s.snap = snap
	s.mu.Unlock()
	return snap, nil
}

// Invalidate — pas ndryshimeve nga admin (instancat e tjera e marrin brenda 30 s).
func (s *Service) Invalidate() {
	s.mu.Lock()
	s.snap = nil
	s.mu.Unlock()
}

// Enabled — a është flag-u aktiv për këtë përdorues? Rollout-i është determinist: hash(key, user) < përqindja.
// Flag i panjohur → false (asgjë nuk aktivizohet pa qenë i deklaruar).
func (s *Service) Enabled(ctx context.Context, key string, userID uuid.UUID) bool {
	snap, err := s.load(ctx)
	if err != nil {
		return false
	}
	f, ok := snap.flags[key]
	if !ok || !f.Enabled {
		return false
	}
	if f.RolloutPercent >= 100 {
		return true
	}
	if f.RolloutPercent <= 0 || userID == uuid.Nil {
		return false
	}
	return bucket(key, userID) < f.RolloutPercent
}

func bucket(key string, userID uuid.UUID) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	h.Write([]byte(":"))
	h.Write(userID[:])
	return int(h.Sum32() % 100)
}

// --- versionet -------------------------------------------------------------------

// CompareVersions — "1.2.3" vs "1.10.0": -1, 0, +1; pjesët jo-numerike trajtohen si 0.
func CompareVersions(a, b string) int {
	pa, pb := parseVersion(a), parseVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func parseVersion(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+ "); i >= 0 {
		v = v[:i]
	}
	for i, p := range strings.SplitN(v, ".", 3) {
		n, _ := strconv.Atoi(p)
		out[i] = n
	}
	return out
}

type ClientInfo struct {
	App      string
	Platform string
	Version  string
}

// ClientFrom — X-App-Id / X-App-Platform / X-App-Version (aplikacionet i dërgojnë gjithmonë).
func ClientFrom(r *http.Request) ClientInfo {
	return ClientInfo{App: strings.ToLower(r.Header.Get("X-App-Id")), Platform: strings.ToLower(r.Header.Get("X-App-Platform")), Version: r.Header.Get("X-App-Version")}
}

// Check — mirëmbajtje → MAINTENANCE; version nën minimum → UPDATE_REQUIRED; klient i panjohur → kalon.
func (s *Service) Check(ctx context.Context, c ClientInfo) error {
	if c.App == "" || c.Platform == "" {
		return nil
	}
	snap, err := s.load(ctx)
	if err != nil {
		return nil // konfigurimi i paarritshëm nuk e bllokon trafikun; logohet nga thirrësi
	}
	v, ok := snap.versions[c.App+"/"+c.Platform]
	if !ok {
		return nil
	}
	if v.Maintenance {
		return httpx.ErrMaintenance
	}
	if c.Version != "" && CompareVersions(c.Version, v.MinVersion) < 0 {
		return httpx.ErrUpdateRequire
	}
	return nil
}

// Gate — middleware: zbaton Check për çdo kërkesë (përveç shëndetit dhe vetë /config).
func (s *Service) Gate() httpx.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := r.URL.Path
			if p == "/healthz" || p == "/readyz" || p == "/api/v1/config" || p == "/api/v1/openapi.yaml" || strings.HasPrefix(p, "/api/v1/payments/webhook/") {
				next.ServeHTTP(w, r)
				return
			}
			if err := s.Check(r.Context(), ClientFrom(r)); err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// --- përgjigja publike ---------------------------------------------------------------

type Public struct {
	App         *AppVersion     `json:"app,omitempty"` // për klientin që pyet (sipas header-ave)
	UpdateState string          `json:"update_state"`  // ok | recommended | required | maintenance | unknown
	Flags       map[string]bool `json:"flags"`         // vetëm flag-et publike, të vlerësuara për përdoruesin (nëse i kyçur)
	ServerTime  time.Time       `json:"server_time"`
	ConfigTTLS  int             `json:"config_ttl_s"`
}

func (s *Service) PublicConfig(ctx context.Context, c ClientInfo, userID uuid.UUID) (*Public, error) {
	snap, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	out := &Public{UpdateState: "unknown", Flags: map[string]bool{}, ServerTime: s.now(), ConfigTTLS: int(CacheTTL.Seconds())}
	if v, ok := snap.versions[c.App+"/"+c.Platform]; ok {
		vv := v
		out.App = &vv
		switch {
		case v.Maintenance:
			out.UpdateState = "maintenance"
		case c.Version != "" && CompareVersions(c.Version, v.MinVersion) < 0:
			out.UpdateState = "required"
		case c.Version != "" && CompareVersions(c.Version, v.RecommendedVersion) < 0:
			out.UpdateState = "recommended"
		default:
			out.UpdateState = "ok"
		}
	}
	for k, f := range snap.flags {
		if f.Public {
			out.Flags[k] = s.Enabled(ctx, k, userID)
		}
	}
	return out, nil
}

// --- admin -------------------------------------------------------------------------

func (s *Service) Flags(ctx context.Context) ([]Flag, error) {
	rows, err := s.pool.Query(ctx, `SELECT key, enabled, rollout_percent, public, description FROM feature_flags ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Flag{}
	for rows.Next() {
		var f Flag
		if err := rows.Scan(&f.Key, &f.Enabled, &f.RolloutPercent, &f.Public, &f.Description); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

type FlagUpdate struct {
	Enabled        *bool `json:"enabled"`
	RolloutPercent *int  `json:"rollout_percent"`
	Public         *bool `json:"public"`
}

func (s *Service) SetFlag(ctx context.Context, admin principal.Actor, key string, in FlagUpdate) (*Flag, error) {
	if in.RolloutPercent != nil && (*in.RolloutPercent < 0 || *in.RolloutPercent > 100) {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"rollout_percent": "invalid"})
	}
	if in.Enabled == nil && in.RolloutPercent == nil && in.Public == nil {
		return nil, httpx.ErrValidation.WithFields(map[string]string{"body": "empty"})
	}
	var f Flag
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			UPDATE feature_flags SET enabled = COALESCE($2, enabled), rollout_percent = COALESCE($3, rollout_percent),
			  public = COALESCE($4, public), updated_by = $5, updated_at = now()
			WHERE key = $1 RETURNING key, enabled, rollout_percent, public, description`, key, in.Enabled, in.RolloutPercent, in.Public, admin.UserID).
			Scan(&f.Key, &f.Enabled, &f.RolloutPercent, &f.Public, &f.Description)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		return audit(ctx, tx, admin, "config.flag_updated", "feature_flag", key, map[string]any{"enabled": f.Enabled, "rollout_percent": f.RolloutPercent, "public": f.Public})
	})
	if err != nil {
		return nil, err
	}
	s.Invalidate()
	return &f, nil
}

type VersionUpdate struct {
	MinVersion         *string `json:"min_version"`
	RecommendedVersion *string `json:"recommended_version"`
	Maintenance        *bool   `json:"maintenance"`
	MaintenanceMessage *string `json:"maintenance_message"`
}

func (s *Service) SetVersion(ctx context.Context, admin principal.Actor, app, platform string, in VersionUpdate) (*AppVersion, error) {
	fields := map[string]string{}
	for name, v := range map[string]*string{"min_version": in.MinVersion, "recommended_version": in.RecommendedVersion} {
		if v != nil && parseVersion(*v) == [3]int{} && strings.TrimSpace(*v) != "0.0.0" {
			fields[name] = "invalid"
		}
	}
	if in.MinVersion == nil && in.RecommendedVersion == nil && in.Maintenance == nil && in.MaintenanceMessage == nil {
		fields["body"] = "empty"
	}
	if len(fields) > 0 {
		return nil, httpx.ErrValidation.WithFields(fields)
	}
	var v AppVersion
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			UPDATE app_versions SET min_version = COALESCE($3, min_version), recommended_version = COALESCE($4, recommended_version),
			  maintenance = COALESCE($5, maintenance), maintenance_message = COALESCE($6, maintenance_message), updated_by = $7, updated_at = now()
			WHERE app = $1 AND platform = $2
			RETURNING app, platform, min_version, recommended_version, maintenance, maintenance_message`,
			app, platform, in.MinVersion, in.RecommendedVersion, in.Maintenance, in.MaintenanceMessage, admin.UserID).
			Scan(&v.App, &v.Platform, &v.MinVersion, &v.RecommendedVersion, &v.Maintenance, &v.MaintenanceMessage)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.ErrNotFound
		}
		if err != nil {
			return err
		}
		return audit(ctx, tx, admin, "config.app_version_updated", "app_version", app+"/"+platform,
			map[string]any{"min_version": v.MinVersion, "recommended_version": v.RecommendedVersion, "maintenance": v.Maintenance})
	})
	if err != nil {
		return nil, err
	}
	s.Invalidate()
	return &v, nil
}

func (s *Service) Versions(ctx context.Context) ([]AppVersion, error) {
	rows, err := s.pool.Query(ctx, `SELECT app, platform, min_version, recommended_version, maintenance, maintenance_message FROM app_versions ORDER BY app, platform`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AppVersion{}
	for rows.Next() {
		var v AppVersion
		if err := rows.Scan(&v.App, &v.Platform, &v.MinVersion, &v.RecommendedVersion, &v.Maintenance, &v.MaintenanceMessage); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func audit(ctx context.Context, tx pgx.Tx, a principal.Actor, action, targetType, targetID string, meta map[string]any) error {
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
