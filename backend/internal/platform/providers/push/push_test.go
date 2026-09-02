package push

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func testServiceAccount(t *testing.T, tokenURI string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	raw, _ := json.Marshal(map[string]string{"project_id": "krejt-test", "client_email": "fcm@krejt-test.iam.gserviceaccount.com", "private_key": pemKey, "token_uri": tokenURI})
	return string(raw)
}

func TestFCMSendAndInvalidToken(t *testing.T) {
	var tokenCalls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&tokenCalls, 1)
		if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" || r.Form.Get("assertion") == "" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "ya29.test", "expires_in": 3600})
	})
	mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer ya29.test" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			Message struct {
				Token        string            `json:"token"`
				Notification map[string]string `json:"notification"`
				Data         map[string]string `json:"data"`
				Android      map[string]any    `json:"android"`
			} `json:"message"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if strings.HasPrefix(body.Message.Token, "dead-") {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"status": "NOT_FOUND", "message": "Requested entity was not found."}})
			return
		}
		if body.Message.Notification["title"] != "Titull" || body.Message.Data["deep_link"] != "krejt://rides/1" || body.Message.Android["priority"] != "high" || body.Message.Android["ttl"] != "20s" {
			http.Error(w, "payload i gabuar", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "projects/krejt-test/messages/123"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f, err := NewFCM(testServiceAccount(t, srv.URL+"/token"))
	if err != nil {
		t.Fatal(err)
	}
	f.endpoint = srv.URL + "/send"
	ctx := context.Background()
	res, err := f.Send(ctx, Message{Token: "live-token-abcdefghijklmnop", Title: "Titull", Body: "Trup", Data: map[string]string{"deep_link": "krejt://rides/1"}, Priority: "high", TTL: 20e9})
	if err != nil || res.ProviderMessageID != "projects/krejt-test/messages/123" || res.InvalidToken {
		t.Fatalf("send: %+v err=%v", res, err)
	}
	res, err = f.Send(ctx, Message{Token: "dead-token-abcdefghijklmnop", Title: "Titull", Body: "Trup", Data: map[string]string{"deep_link": "krejt://rides/1"}, Priority: "high", TTL: 20e9})
	if err == nil || !res.InvalidToken {
		t.Fatalf("token i vdekur: %+v err=%v", res, err)
	}
	if atomic.LoadInt32(&tokenCalls) != 1 {
		t.Fatalf("token-i OAuth duhej marrë një herë, u mor %d herë", tokenCalls)
	}
	if got := shorten("abcdefghijklmnopqrstuvwxyz"); got != "abcdef…wxyz" {
		t.Fatalf("shorten: %q", got)
	}
}

func TestNewFromEnvGuards(t *testing.T) {
	if _, err := NewFromEnv("production", "devlog", "", nil); err == nil {
		t.Fatal("devlog në production duhej refuzuar")
	}
	if _, err := NewFromEnv("production", "fcm", "", nil); err == nil {
		t.Fatal("fcm pa llogari shërbimi duhej refuzuar")
	}
	if _, err := NewFromEnv("production", "smoke-signals", "", nil); err == nil {
		t.Fatal("ofrues i panjohur duhej refuzuar")
	}
}
