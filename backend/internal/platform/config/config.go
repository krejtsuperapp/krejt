// Package config lexon konfigurimin nga mjedisi (§73). Asnjë vlerë e ngurtësuar, asnjë sekret në kod.
// Në ECS, kredencialet e Aurora-s vijnë nga Secrets Manager si JSON (DB_CREDENTIALS_JSON);
// lokalisht (docker-compose) përdoret DATABASE_URL direkt.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Redis struct {
	Host     string // host:port (ElastiCache configuration endpoint ose localhost:6379)
	Password string
	TLS      bool
}

type Config struct {
	Env      string // development | staging | production
	Version  string
	Region   string
	HTTPPort string

	databaseURL string
	dbWriter    string
	dbName      string
	dbCreds     string // JSON {"username":"...","password":"..."} nga Secrets Manager

	Redis     Redis
	QueueURLs map[string]string

	// autentikimi (§53) — vlerat nga Secrets Manager (krejt/<env>/jwt, krejt/<env>/infobip)
	JWTPrivateKeyPEM string
	OTPPepper        string
	// Numri që merr SUPER_ADMIN në nisje, por vetëm nëse sistemi ende nuk ka asnjë administrator.
	// Pa këtë, të drejtat e stafit nuk lindin kurrë: jepen vetëm nga një administrator ekzistues.
	BootstrapAdminPhone string

	// A kërkohen dokumentet e miratuara para se një shofer të aprovohet. Fikja lejohet vetëm
	// në development: në prodhim do të thoshte shoferë në rrugë pa patentë të verifikuar.
	DocumentsRequired bool

	// Numra prove (E.164, të ndarë me presje) që kyçen me një kod fiks, pa SMS dhe pa skadim,
	// dhe marrin SUPER_ADMIN në kyçje. Vetëm në development: kudo tjetër serveri nuk niset.
	DevTestPhones []string
	DevTestOTP    string

	SMSProvider    string // infobip | devlog (vetëm development)
	InfobipBaseURL string
	InfobipAPIKey  string
	InfobipSender  string

	// ngjarjet e domenit (§41): EVENTS_PUBLISHER sns | devlog (vetëm development)
	EventsPublisher      string
	DomainEventsTopicARN string

	// hartat (§46): MAPS_PROVIDER google | mapbox | devestimate (vetëm development).
	// Çelësat vijnë nga Secrets Manager: krejt/<env>/google-maps ose krejt/<env>/mapbox-token.
	MapsProvider  string
	GoogleMapsKey string
	MapboxToken   string

	// push (§47): PUSH_PROVIDER fcm | devlog (vetëm development); llogaria e shërbimit nga Secrets Manager (krejt/<env>/fcm)
	PushProvider          string
	FCMServiceAccountJSON string

	// realtime (§42): REALTIME_PROVIDER centrifugo | devlog (vetëm development); sekretet nga krejt/<env>/centrifugo
	RealtimeProvider          string
	CentrifugoAPIURL          string
	CentrifugoAPIKey          string
	CentrifugoTokenHMACSecret string

	// ruajtja e objekteve (§43 S3): STORAGE_PROVIDER s3 | devfs (vetëm development)
	StorageProvider string
	AssetsBucket    string
	DevFSDir        string
	PublicBaseURL   string // URL-ja publike e API-së (për devfs dhe linqe)

	// pagesat (§24): PAYMENT_PROVIDER stripe | devlog (vetëm development); sekretet nga krejt/<env>/payment-provider
	PaymentProvider     string
	StripeSecretKey     string
	StripeWebhookSecret string

	// observability (§50) dhe analitika (§66)
	SentryDSN         string
	AnalyticsProvider string // posthog | devlog (vetëm development)
	PostHogKey        string
	PostHogHost       string
}

func Load() (*Config, error) {
	c := &Config{
		Env:         getenv("APP_ENV", "development"),
		Version:     getenv("APP_VERSION", "dev"),
		Region:      getenv("AWS_REGION", "eu-central-1"),
		HTTPPort:    getenv("HTTP_PORT", "8080"),
		databaseURL: os.Getenv("DATABASE_URL"),
		dbWriter:    os.Getenv("DB_WRITER_HOST"),
		dbName:      getenv("DB_NAME", "krejt"),
		dbCreds:     os.Getenv("DB_CREDENTIALS_JSON"),
		Redis: Redis{
			Host:     getenv("REDIS_HOST", "localhost:6379"),
			Password: os.Getenv("REDIS_AUTH"),
			TLS:      strings.EqualFold(getenv("REDIS_TLS", "false"), "true"),
		},
		QueueURLs:                 map[string]string{},
		JWTPrivateKeyPEM:          os.Getenv("JWT_PRIVATE_KEY"),
		OTPPepper:                 os.Getenv("OTP_PEPPER"),
		BootstrapAdminPhone:       os.Getenv("BOOTSTRAP_ADMIN_PHONE"),
		DocumentsRequired:         getenv("DOCUMENTS_REQUIRED", "true") != "false",
		DevTestPhones:             splitList(os.Getenv("DEV_TEST_PHONES")),
		DevTestOTP:                os.Getenv("DEV_TEST_OTP"),
		SMSProvider:               getenv("SMS_PROVIDER", "infobip"),
		InfobipBaseURL:            getenv("INFOBIP_BASE_URL", "https://api.infobip.com"),
		InfobipAPIKey:             os.Getenv("INFOBIP_API_KEY"),
		InfobipSender:             getenv("INFOBIP_SENDER", "KREJT"),
		EventsPublisher:           getenv("EVENTS_PUBLISHER", "sns"),
		DomainEventsTopicARN:      os.Getenv("SNS_DOMAIN_EVENTS_TOPIC_ARN"),
		MapsProvider:              getenv("MAPS_PROVIDER", "google"),
		GoogleMapsKey:             os.Getenv("GOOGLE_MAPS_KEY"),
		MapboxToken:               os.Getenv("MAPBOX_TOKEN"),
		PushProvider:              getenv("PUSH_PROVIDER", "fcm"),
		FCMServiceAccountJSON:     os.Getenv("FCM_SERVICE_ACCOUNT_JSON"),
		RealtimeProvider:          getenv("REALTIME_PROVIDER", "centrifugo"),
		CentrifugoAPIURL:          os.Getenv("CENTRIFUGO_API_URL"),
		CentrifugoAPIKey:          os.Getenv("CENTRIFUGO_API_KEY"),
		CentrifugoTokenHMACSecret: os.Getenv("CENTRIFUGO_TOKEN_HMAC_SECRET"),
		StorageProvider:           getenv("STORAGE_PROVIDER", "s3"),
		AssetsBucket:              os.Getenv("S3_ASSETS_BUCKET"),
		DevFSDir:                  os.Getenv("DEVFS_DIR"),
		PublicBaseURL:             getenv("PUBLIC_BASE_URL", "http://localhost:8080"),
		PaymentProvider:           getenv("PAYMENT_PROVIDER", "stripe"),
		StripeSecretKey:           os.Getenv("STRIPE_SECRET_KEY"),
		StripeWebhookSecret:       os.Getenv("STRIPE_WEBHOOK_SECRET"),
		SentryDSN:                 os.Getenv("SENTRY_DSN"),
		AnalyticsProvider:         getenv("ANALYTICS_PROVIDER", "posthog"),
		PostHogKey:                os.Getenv("POSTHOG_KEY"),
		PostHogHost:               getenv("POSTHOG_HOST", "https://eu.i.posthog.com"),
	}
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "SQS_") && strings.Contains(kv, "_QUEUE_URL=") {
			parts := strings.SplitN(kv, "=", 2)
			name := strings.ToLower(strings.TrimSuffix(strings.TrimPrefix(parts[0], "SQS_"), "_QUEUE_URL"))
			c.QueueURLs[name] = parts[1]
		}
	}
	if !strings.Contains(c.Redis.Host, ":") {
		c.Redis.Host += ":6379"
	}
	if c.databaseURL == "" && c.dbWriter == "" {
		return nil, errors.New("config: DATABASE_URL ose DB_WRITER_HOST duhet të jetë i vendosur")
	}
	switch c.Env {
	case "development", "staging", "production":
	default:
		return nil, fmt.Errorf("config: APP_ENV i panjohur %q", c.Env)
	}
	// I njëjti kufi si te ofruesit e provës: lehtësimet e dev-it nuk kalojnë dot më tej.
	if !c.DocumentsRequired && c.Env != "development" {
		return nil, errors.New("config: DOCUMENTS_REQUIRED=false lejohet vetëm në development")
	}
	if (len(c.DevTestPhones) > 0 || c.DevTestOTP != "") && c.Env != "development" {
		return nil, errors.New("config: DEV_TEST_PHONES / DEV_TEST_OTP lejohen vetëm në development")
	}
	if len(c.DevTestPhones) > 0 && len(c.DevTestOTP) < 6 {
		return nil, errors.New("config: DEV_TEST_OTP duhet të ketë të paktën 6 shifra")
	}
	return c, nil
}

// DatabaseDSN ndërton DSN-në pa e logaruar kurrë fjalëkalimin.
func (c *Config) DatabaseDSN() string {
	if c.databaseURL != "" {
		return c.databaseURL
	}
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = json.Unmarshal([]byte(c.dbCreds), &creds)
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(creds.Username, creds.Password),
		Host:     c.dbWriter + ":5432",
		Path:     "/" + c.dbName,
		RawQuery: "sslmode=require&application_name=krejt-" + c.Env,
	}
	return u.String()
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

// splitList ndan një listë me presje dhe heq hapësirat; bosh → asnjë element.
func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
