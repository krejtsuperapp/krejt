package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"krejt.app/backend/internal/platform/logx"
)

func TestCentrifugoPublish(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/publish" || r.Header.Get("X-API-Key") != "k-test" || r.Method != http.MethodPost {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		var body struct {
			Channel string         `json:"channel"`
			Data    map[string]any `json:"data"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Channel == "ride:boom" {
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": 102, "message": "unknown channel"}})
			return
		}
		if body.Channel != "ride:1" || body.Data["type"] != "RideAssigned" {
			http.Error(w, "payload", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{}})
	}))
	defer srv.Close()
	c, err := NewCentrifugo(srv.URL+"/api/", "k-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Publish(context.Background(), "ride:1", map[string]any{"type": "RideAssigned"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Publish(context.Background(), "ride:boom", map[string]any{"type": "x"}); err == nil {
		t.Fatal("gabimi i Centrifugo-s duhej kthyer")
	}
	if _, err := NewCentrifugo("", "k"); err == nil {
		t.Fatal("pa URL duhej refuzuar")
	}
}

func TestTokens(t *testing.T) {
	log := logx.New("test", "development")
	if _, err := NewTokenIssuer("production", "", log); err == nil {
		t.Fatal("prod pa sekret duhej refuzuar")
	}
	if _, err := NewTokenIssuer("production", "short", log); err == nil {
		t.Fatal("sekret i shkurtër duhej refuzuar")
	}
	iss, err := NewTokenIssuer("development", "", log) // i përkohshëm
	if err != nil {
		t.Fatal(err)
	}
	tok, exp, err := iss.ConnectionToken("user-1", time.Hour)
	if err != nil || time.Until(exp) < 59*time.Minute {
		t.Fatalf("connection token: %v %v", err, exp)
	}
	claims, err := iss.Parse(tok)
	if err != nil || claims["sub"] != "user-1" || claims["channel"] != nil {
		t.Fatalf("claims: %v %v", claims, err)
	}
	sub, _, err := iss.SubscriptionToken("user-1", "ride:abc", 2*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err = iss.Parse(sub)
	if err != nil || claims["channel"] != "ride:abc" || claims["sub"] != "user-1" {
		t.Fatalf("sub claims: %v %v", claims, err)
	}
	other, _ := NewTokenIssuer("development", "another-secret-that-is-long", log)
	if _, err := other.Parse(tok); err == nil {
		t.Fatal("sekret tjetër duhej ta refuzonte token-in")
	}
	if _, err := NewFromEnv("production", "devlog", "", "", log); err == nil {
		t.Fatal("devlog në production duhej refuzuar")
	}
}
