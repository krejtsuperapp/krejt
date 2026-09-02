// Package realtime — RealtimeProvider (§42): Centrifugo (HTTP API për publikim; JWT HS256 për lidhje
// dhe abonim). Klientët nuk bëjnë polling: gjendja e udhëtimit, pozicioni i shoferit dhe ofertat
// vijnë përmes kanaleve. Autorizimi i kanaleve bëhet në API (token abonimi), kurrë në klient.
package realtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Provider interface {
	Publish(ctx context.Context, channel string, data any) error
}

// --- Centrifugo HTTP API ---------------------------------------------------------

type Centrifugo struct {
	apiURL string // p.sh. http://centrifugo.krejt-dev.local:8000/api
	apiKey string
	http   *http.Client
}

func NewCentrifugo(apiURL, apiKey string) (*Centrifugo, error) {
	apiURL = strings.TrimRight(strings.TrimSpace(apiURL), "/")
	if apiURL == "" || apiKey == "" {
		return nil, errors.New("realtime: CENTRIFUGO_API_URL / CENTRIFUGO_API_KEY mungojnë")
	}
	return &Centrifugo{apiURL: apiURL, apiKey: apiKey, http: &http.Client{Timeout: 4 * time.Second}}, nil
}

func (c *Centrifugo) Publish(ctx context.Context, channel string, data any) error {
	raw, err := json.Marshal(map[string]any{"channel": channel, "data": data})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiURL+"/publish", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("realtime: publish %s: %w", channel, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("realtime: publish %s: HTTP %d", channel, resp.StatusCode)
	}
	var out struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err == nil && out.Error != nil {
		return fmt.Errorf("realtime: publish %s: centrifugo %d %s", channel, out.Error.Code, out.Error.Message)
	}
	return nil
}

// --- DevLog (VETËM development) -------------------------------------------------

type DevLog struct{ log *slog.Logger }

func (d *DevLog) Publish(_ context.Context, channel string, data any) error {
	d.log.Info("DEV ONLY — realtime publish (no Centrifugo)", "channel", channel, "data", data)
	return nil
}

// NewFromEnv — REALTIME_PROVIDER: centrifugo (parazgjedhje) | devlog (vetëm development).
func NewFromEnv(env, provider, apiURL, apiKey string, log *slog.Logger) (Provider, error) {
	switch provider {
	case "centrifugo", "":
		return NewCentrifugo(apiURL, apiKey)
	case "devlog":
		if env != "development" {
			return nil, fmt.Errorf("realtime: devlog lejohet vetëm në development (APP_ENV=%s)", env)
		}
		log.Warn("DEV ONLY — REALTIME_PROVIDER=devlog: publikimet vetëm logohen")
		return &DevLog{log: log}, nil
	default:
		return nil, fmt.Errorf("realtime: ofrues i panjohur %q", provider)
	}
}

// --- token-at (JWT HS256, i njëjti sekret si Centrifugo `token_hmac_secret_key`) -------------

type TokenIssuer struct {
	secret []byte
}

// NewTokenIssuer — sekreti nga Secrets Manager; në development, nëse mungon, gjenerohet i përkohshëm
// (token-at s'do të pranohen nga një Centrifugo real — vetëm me REALTIME_PROVIDER=devlog).
func NewTokenIssuer(env, secret string, log *slog.Logger) (*TokenIssuer, error) {
	if secret == "" {
		if env != "development" {
			return nil, errors.New("realtime: CENTRIFUGO_TOKEN_HMAC_SECRET mungon")
		}
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		secret = hex.EncodeToString(b)
		log.Warn("DEV ONLY — CENTRIFUGO_TOKEN_HMAC_SECRET mungon: sekret i përkohshëm (token-at vlejnë vetëm në këtë proces)")
	}
	if len(secret) < 16 {
		return nil, errors.New("realtime: CENTRIFUGO_TOKEN_HMAC_SECRET shumë i shkurtër (>= 16 byte)")
	}
	return &TokenIssuer{secret: []byte(secret)}, nil
}

// ConnectionToken — token lidhjeje për përdoruesin (sub = user id).
func (t *TokenIssuer) ConnectionToken(userID string, ttl time.Duration) (string, time.Time, error) {
	exp := time.Now().Add(ttl)
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": userID, "exp": exp.Unix(), "iat": time.Now().Unix()}).SignedString(t.secret)
	return tok, exp, err
}

// SubscriptionToken — token abonimi për një kanal të autorizuar nga serveri.
func (t *TokenIssuer) SubscriptionToken(userID, channel string, ttl time.Duration) (string, time.Time, error) {
	exp := time.Now().Add(ttl)
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": userID, "channel": channel, "exp": exp.Unix(), "iat": time.Now().Unix()}).SignedString(t.secret)
	return tok, exp, err
}

// Parse — për teste dhe diagnostikë: verifikon nënshkrimin dhe kthen pretendimet.
func (t *TokenIssuer) Parse(token string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}))
	return claims, err
}
