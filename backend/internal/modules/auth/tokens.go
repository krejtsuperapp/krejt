// Tokenat (§53): access token JWT RS256 jetëshkurtër (15 min) me kapacitetet brenda;
// refresh token i rastësishëm 256-bit, i ruajtur vetëm si SHA-256, me rotacion në çdo përdorim.
package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	Issuer          = "krejt"
	Audience        = "krejt-api"
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 30 * 24 * time.Hour
)

type Claims struct {
	Capabilities []string `json:"caps"`
	SessionID    string   `json:"sid"`
	jwt.RegisteredClaims
}

type Signer struct {
	priv *rsa.PrivateKey
	pub  *rsa.PublicKey
	kid  string
}

// LoadSigner lexon çelësin privat PEM (PKCS#1 ose PKCS#8). Publiku nxirret prej tij.
func LoadSigner(privatePEM []byte) (*Signer, error) {
	block, _ := pem.Decode(privatePEM)
	if block == nil {
		return nil, errors.New("auth: invalid private key PEM")
	}
	var key *rsa.PrivateKey
	if k, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		key = k
	} else if k8, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		rk, ok := k8.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("auth: private key is not RSA")
		}
		key = rk
	} else {
		return nil, fmt.Errorf("auth: parse private key: %w", err)
	}
	if key.N.BitLen() < 2048 {
		return nil, errors.New("auth: RSA key must be >= 2048 bits")
	}
	return newSigner(key), nil
}

// GenerateEphemeralSigner — VETËM development: çelës i ri në çdo nisje (tokenat s'mbijetojnë restart-in).
func GenerateEphemeralSigner() (*Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return newSigner(key), nil
}

func newSigner(key *rsa.PrivateKey) *Signer {
	sum := sha256.Sum256(x509.MarshalPKCS1PublicKey(&key.PublicKey))
	return &Signer{priv: key, pub: &key.PublicKey, kid: base64.RawURLEncoding.EncodeToString(sum[:8])}
}

func (s *Signer) IssueAccess(userID, sessionID uuid.UUID, caps []string, now time.Time) (string, error) {
	claims := Claims{
		Capabilities: caps,
		SessionID:    sessionID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Audience:  jwt.ClaimStrings{Audience},
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenTTL)),
			ID:        uuid.NewString(),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = s.kid
	return tok.SignedString(s.priv)
}

func (s *Signer) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("auth: unexpected signing method")
		}
		return s.pub, nil
	}, jwt.WithIssuer(Issuer), jwt.WithAudience(Audience), jwt.WithLeeway(30*time.Second))
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// NewRefreshToken kthen token-in e pastër (për klientin) dhe hash-in (për DB).
func NewRefreshToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}
