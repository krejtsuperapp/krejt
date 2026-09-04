// Package push — PushProvider (§47): Firebase Cloud Messaging HTTP v1 (Android + iOS përmes APNs).
// Kredencialet: llogaria e shërbimit (JSON) nga Secrets Manager; token OAuth2 i gjeneruar me RS256
// dhe i ruajtur deri në skadim. Token-at e pavlefshëm raportohen që të hiqen nga baza.
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Message — një njoftim për një pajisje. Data udhëton gjithmonë (deep link, çelësat); Title/Body
// janë tekst i përkthyer sipas gjuhës së pajisjes.
type Message struct {
	Token    string
	Title    string
	Body     string
	Data     map[string]string
	Priority string        // high | normal
	TTL      time.Duration // 0 = parazgjedhja e ofruesit
	Collapse string        // çelës grumbullimi (p.sh. ride:{id})
}

type Result struct {
	ProviderMessageID string
	InvalidToken      bool // token-i duhet çaktivizuar (UNREGISTERED / i pavlefshëm)
}

type Provider interface {
	Send(ctx context.Context, m Message) (Result, error)
}

// --- FCM HTTP v1 ---------------------------------------------------------------

type serviceAccount struct {
	ProjectID   string `json:"project_id"`
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

type FCM struct {
	sa       serviceAccount
	http     *http.Client
	endpoint string // override në teste

	mu     sync.Mutex
	token  string
	expiry time.Time
}

func NewFCM(serviceAccountJSON string) (*FCM, error) {
	var sa serviceAccount
	if err := json.Unmarshal([]byte(serviceAccountJSON), &sa); err != nil {
		return nil, fmt.Errorf("push: FCM_SERVICE_ACCOUNT_JSON: %w", err)
	}
	if sa.ProjectID == "" || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, errors.New("push: llogaria e shërbimit FCM pa project_id/client_email/private_key")
	}
	if sa.TokenURI == "" {
		sa.TokenURI = "https://oauth2.googleapis.com/token"
	}
	return &FCM{sa: sa, http: &http.Client{Timeout: 8 * time.Second},
		endpoint: "https://fcm.googleapis.com/v1/projects/" + sa.ProjectID + "/messages:send"}, nil
}

func (f *FCM) accessToken(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.token != "" && time.Now().Before(f.expiry.Add(-time.Minute)) {
		return f.token, nil
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(f.sa.PrivateKey))
	if err != nil {
		return "", fmt.Errorf("push: çelësi privat i FCM: %w", err)
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"iss":   f.sa.ClientEmail,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   f.sa.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	}
	assertion, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		return "", err
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {assertion}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, f.sa.TokenURI, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := f.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("push: oauth2: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("push: oauth2: HTTP %d", resp.StatusCode)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil || tok.AccessToken == "" {
		return "", errors.New("push: oauth2: përgjigje pa access_token")
	}
	f.token = tok.AccessToken
	f.expiry = now.Add(time.Duration(tok.ExpiresIn) * time.Second)
	return f.token, nil
}

func (f *FCM) Send(ctx context.Context, m Message) (Result, error) {
	tok, err := f.accessToken(ctx)
	if err != nil {
		return Result{}, err
	}
	priority := "high"
	apnsPriority := "10"
	if m.Priority == "normal" {
		priority, apnsPriority = "normal", "5"
	}
	android := map[string]any{"priority": priority}
	apnsHeaders := map[string]string{"apns-priority": apnsPriority}
	if m.TTL > 0 {
		android["ttl"] = fmt.Sprintf("%ds", int(m.TTL.Seconds()))
		apnsHeaders["apns-expiration"] = fmt.Sprintf("%d", time.Now().Add(m.TTL).Unix())
	}
	if m.Collapse != "" {
		android["collapse_key"] = m.Collapse
		apnsHeaders["apns-collapse-id"] = m.Collapse
	}
	msg := map[string]any{
		"token":   m.Token,
		"data":    m.Data,
		"android": android,
		"apns":    map[string]any{"headers": apnsHeaders, "payload": map[string]any{"aps": map[string]any{"sound": "default", "mutable-content": 1}}},
	}
	if m.Title != "" || m.Body != "" {
		msg["notification"] = map[string]string{"title": m.Title, "body": m.Body}
	}
	raw, _ := json.Marshal(map[string]any{"message": msg})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, f.endpoint, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := f.http.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("push: fcm send: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode == http.StatusOK {
		var out struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(body, &out)
		return Result{ProviderMessageID: out.Name}, nil
	}
	// UNREGISTERED (404) / INVALID_ARGUMENT për token (400) → token-i hiqet; të tjerat riprovohen
	var ferr struct {
		Error struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &ferr)
	if resp.StatusCode == http.StatusNotFound || ferr.Error.Status == "UNREGISTERED" ||
		(ferr.Error.Status == "INVALID_ARGUMENT" && strings.Contains(strings.ToLower(ferr.Error.Message), "token")) {
		return Result{InvalidToken: true}, fmt.Errorf("push: fcm: %s", nonEmpty(ferr.Error.Status, "UNREGISTERED"))
	}
	return Result{}, fmt.Errorf("push: fcm: HTTP %d %s", resp.StatusCode, ferr.Error.Status)
}

func nonEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// --- DevLog (VETËM development) -----------------------------------------------

type DevLog struct{ log *slog.Logger }

func (d *DevLog) Send(_ context.Context, m Message) (Result, error) {
	d.log.Info("DEV ONLY — push (not delivered)", "token", shorten(m.Token), "title", m.Title, "body", m.Body, "data", m.Data)
	return Result{ProviderMessageID: "devlog"}, nil
}

// NewFromEnv — PUSH_PROVIDER: fcm (parazgjedhje) | devlog (development dhe staging; kurrë production).
func NewFromEnv(env, provider, serviceAccountJSON string, log *slog.Logger) (Provider, error) {
	switch provider {
	case "fcm", "":
		if serviceAccountJSON == "" {
			return nil, errors.New("push: FCM_SERVICE_ACCOUNT_JSON mungon")
		}
		return NewFCM(serviceAccountJSON)
	case "devlog":
		if env == "production" {
			return nil, fmt.Errorf("push: devlog nuk lejohet në production (APP_ENV=%s)", env)
		}
		log.Warn("DEV ONLY — PUSH_PROVIDER=devlog: push-et vetëm logohen")
		return &DevLog{log: log}, nil
	default:
		return nil, fmt.Errorf("push: ofrues i panjohur %q", provider)
	}
}

// shorten — token-i kurrë i plotë në log (§51).
func shorten(t string) string {
	if len(t) <= 12 {
		return "…"
	}
	return t[:6] + "…" + t[len(t)-4:]
}

// Shorten — për ruajtje si "target" në gjurmën e dorëzimit.
func Shorten(t string) string { return shorten(t) }
