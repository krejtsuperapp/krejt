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
	SMSProvider      string // infobip | devlog (vetëm development)
	InfobipBaseURL   string
	InfobipAPIKey    string
	InfobipSender    string

	// ngjarjet e domenit (§41): EVENTS_PUBLISHER sns | devlog (vetëm development)
	EventsPublisher      string
	DomainEventsTopicARN string

	// hartat (§46): MAPS_PROVIDER google | devestimate (vetëm development); çelësi nga Secrets Manager (krejt/<env>/google-maps)
	MapsProvider  string
	GoogleMapsKey string

	// push (§47): PUSH_PROVIDER fcm | devlog (vetëm development); llogaria e shërbimit nga Secrets Manager (krejt/<env>/fcm)
	PushProvider          string
	FCMServiceAccountJSON string

	// realtime (§42): REALTIME_PROVIDER centrifugo | devlog (vetëm development); sekretet nga krejt/<env>/centrifugo
	RealtimeProvider          string
	CentrifugoAPIURL          string
	CentrifugoAPIKey          string
	CentrifugoTokenHMACSecret string
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
		SMSProvider:               getenv("SMS_PROVIDER", "infobip"),
		InfobipBaseURL:            getenv("INFOBIP_BASE_URL", "https://api.infobip.com"),
		InfobipAPIKey:             os.Getenv("INFOBIP_API_KEY"),
		InfobipSender:             getenv("INFOBIP_SENDER", "KREJT"),
		EventsPublisher:           getenv("EVENTS_PUBLISHER", "sns"),
		DomainEventsTopicARN:      os.Getenv("SNS_DOMAIN_EVENTS_TOPIC_ARN"),
		MapsProvider:              getenv("MAPS_PROVIDER", "google"),
		GoogleMapsKey:             os.Getenv("GOOGLE_MAPS_KEY"),
		PushProvider:              getenv("PUSH_PROVIDER", "fcm"),
		FCMServiceAccountJSON:     os.Getenv("FCM_SERVICE_ACCOUNT_JSON"),
		RealtimeProvider:          getenv("REALTIME_PROVIDER", "centrifugo"),
		CentrifugoAPIURL:          os.Getenv("CENTRIFUGO_API_URL"),
		CentrifugoAPIKey:          os.Getenv("CENTRIFUGO_API_KEY"),
		CentrifugoTokenHMACSecret: os.Getenv("CENTRIFUGO_TOKEN_HMAC_SECRET"),
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

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
