// Package sms — SmsProvider (§48): Infobip në prodhim, pas një ndërfaqeje që lejon zëvendësim.
// Kredencialet vijnë vetëm nga mjedisi/Secrets Manager. Ofruesi "devlog" ekziston VETËM për
// APP_ENV=development (§75: mock vetëm pas ndërfaqeve eksplicite të zhvillimit).
package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Provider interface {
	SendOTP(ctx context.Context, phoneE164, code, locale string) error
}

// Message kthen tekstin e OTP-së në gjuhën e përdoruesit (§2: asnjë varg i ngurtësuar në UI;
// këtu janë tekste transaksionale të serverit, të grupuara në një vend për përkthim).
func Message(locale, code string) string {
	switch locale {
	case "en":
		return fmt.Sprintf("Your KREJT code is %s. It expires in 5 minutes. Never share it.", code)
	case "de":
		return fmt.Sprintf("Dein KREJT-Code ist %s. Er läuft in 5 Minuten ab. Gib ihn niemals weiter.", code)
	default:
		return fmt.Sprintf("Kodi yt KREJT është %s. Skadon pas 5 minutash. Mos ia jep askujt.", code)
	}
}

// ---------------------------------------------------------------------------
// Infobip — https://www.infobip.com/docs/api/channels/sms/sms-messaging/outbound-sms/send-sms-message
// ---------------------------------------------------------------------------
type Infobip struct {
	baseURL string
	apiKey  string
	sender  string
	client  *http.Client
}

func NewInfobip(baseURL, apiKey, sender string) (*Infobip, error) {
	if baseURL == "" || apiKey == "" || sender == "" {
		return nil, errors.New("sms: infobip requires INFOBIP_BASE_URL, INFOBIP_API_KEY, INFOBIP_SENDER")
	}
	return &Infobip{baseURL: baseURL, apiKey: apiKey, sender: sender, client: &http.Client{Timeout: 10 * time.Second}}, nil
}

func (p *Infobip) SendOTP(ctx context.Context, phoneE164, code, locale string) error {
	body, _ := json.Marshal(map[string]any{
		"messages": []map[string]any{{
			"from":         p.sender,
			"destinations": []map[string]string{{"to": phoneE164}},
			"text":         Message(locale, code),
		}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/sms/2/text/advanced", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "App "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("sms: infobip request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("sms: infobip status %d: %s", res.StatusCode, string(b))
	}
	return nil
}

// ---------------------------------------------------------------------------
// DevLog — VETËM development: nuk dërgon asgjë, shkruan kodin në log lokal.
// Në staging/prod ndërtimi refuzon ta përdorë (shih NewFromEnv).
// ---------------------------------------------------------------------------
type DevLog struct{ log *slog.Logger }

func NewDevLog(log *slog.Logger) *DevLog { return &DevLog{log: log} }

func (d *DevLog) SendOTP(_ context.Context, phoneE164, code, locale string) error {
	d.log.Warn("DEV ONLY — OTP not sent, printed locally", "phone", phoneE164, "dev_only_code", code, "locale", locale)
	return nil
}

// NewFromEnv zgjedh ofruesin: SMS_PROVIDER=infobip (parazgjedhje) ose devlog (vetëm development).
func NewFromEnv(env, provider, infobipBaseURL, infobipKey, infobipSender string, log *slog.Logger) (Provider, error) {
	switch provider {
	case "", "infobip":
		return NewInfobip(infobipBaseURL, infobipKey, infobipSender)
	case "devlog":
		if env != "development" {
			return nil, errors.New("sms: devlog provider is allowed only in development")
		}
		return NewDevLog(log), nil
	default:
		return nil, fmt.Errorf("sms: unknown provider %q", provider)
	}
}
