// Package analytics — AnalyticsProvider (§66): ngjarje produkti drejt PostHog (EU). distinct_id është
// id-ja e përdoruesit (uuid), kurrë telefon/email; vetitë mbajnë vetëm çelësa biznesi (kategori, shuma,
// gjendje). Dërgim në grup (batch) asinkron; devlog vetëm në development.
package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Event struct {
	DistinctID string         `json:"distinct_id"`
	Event      string         `json:"event"`
	Properties map[string]any `json:"properties,omitempty"`
	Timestamp  time.Time      `json:"timestamp"`
}

type Provider interface {
	Capture(ev Event)
	Close(ctx context.Context) error
}

// --- PostHog ------------------------------------------------------------------------------

type PostHog struct {
	key   string
	host  string
	http  *http.Client
	log   *slog.Logger
	mu    sync.Mutex
	buf   []Event
	stop  chan struct{}
	done  chan struct{}
	every time.Duration
	max   int
}

func NewPostHog(key, host string, log *slog.Logger) (*PostHog, error) {
	if key == "" {
		return nil, errors.New("analytics: POSTHOG_KEY mungon")
	}
	host = strings.TrimRight(host, "/")
	if host == "" {
		host = "https://eu.i.posthog.com"
	}
	p := &PostHog{key: key, host: host, http: &http.Client{Timeout: 8 * time.Second}, log: log,
		stop: make(chan struct{}), done: make(chan struct{}), every: 5 * time.Second, max: 50}
	go p.loop()
	return p, nil
}

func (p *PostHog) Capture(ev Event) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now()
	}
	p.mu.Lock()
	p.buf = append(p.buf, ev)
	full := len(p.buf) >= p.max
	p.mu.Unlock()
	if full {
		p.flush(context.Background())
	}
}

func (p *PostHog) loop() {
	defer close(p.done)
	t := time.NewTicker(p.every)
	defer t.Stop()
	for {
		select {
		case <-p.stop:
			return
		case <-t.C:
			p.flush(context.Background())
		}
	}
}

func (p *PostHog) flush(ctx context.Context) {
	p.mu.Lock()
	batch := p.buf
	p.buf = nil
	p.mu.Unlock()
	if len(batch) == 0 {
		return
	}
	body, _ := json.Marshal(map[string]any{"api_key": p.key, "batch": batch})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.host+"/batch/", bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.http.Do(req)
	if err != nil {
		p.log.Warn("analytics: batch failed", "count", len(batch), "err", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		p.log.Warn("analytics: batch rejected", "count", len(batch), "status", resp.StatusCode)
	}
}

func (p *PostHog) Close(ctx context.Context) error {
	close(p.stop)
	<-p.done
	p.flush(ctx)
	return nil
}

// --- DevLog (VETËM development) -----------------------------------------------------------

type DevLog struct{ log *slog.Logger }

func (d *DevLog) Capture(ev Event) {
	d.log.Info("DEV ONLY — analytics event (not sent)", "event", ev.Event, "distinct_id", ev.DistinctID, "props", ev.Properties)
}
func (d *DevLog) Close(context.Context) error { return nil }

// NewFromEnv — ANALYTICS_PROVIDER: posthog (parazgjedhje) | devlog (development dhe staging; kurrë production).
func NewFromEnv(env, provider, key, host string, log *slog.Logger) (Provider, error) {
	switch provider {
	case "posthog", "":
		return NewPostHog(key, host, log)
	case "devlog":
		if env == "production" {
			return nil, fmt.Errorf("analytics: devlog nuk lejohet në production (APP_ENV=%s)", env)
		}
		log.Warn("DEV ONLY — ANALYTICS_PROVIDER=devlog: ngjarjet vetëm logohen")
		return &DevLog{log: log}, nil
	default:
		return nil, fmt.Errorf("analytics: ofrues i panjohur %q", provider)
	}
}
