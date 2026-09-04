// Package storage — StorageProvider (§43 S3): ngarkim direkt nga klienti me URL të nënshkruar (PUT),
// verifikim pas ngarkimit (HEAD: madhësia dhe lloji përputhen me atë që u premtua), lexim me URL të
// nënshkruar jetëshkurtër. Bucket-i është privat; asnjë objekt publik.
// `devfs` (VETËM development): ruajtje në disk lokal përmes një endpoint-i të API-së.
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type ObjectInfo struct {
	Key         string
	ContentType string
	SizeBytes   int64
}

type UploadTarget struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

var ErrNotFound = errors.New("storage: object not found")

type Provider interface {
	PresignUpload(ctx context.Context, key, contentType string, sizeBytes int64, ttl time.Duration) (UploadTarget, error)
	// PutBytes — shkrim nga vetë serveri, për objekte që i prodhon ai (p.sh. eksporti i të dhënave).
	// Ngarkimet e përdoruesve kalojnë gjithmonë nga PresignUpload, që bajtët të mos prekin API-në.
	PutBytes(ctx context.Context, key, contentType string, body []byte) error
	Head(ctx context.Context, key string) (ObjectInfo, error)
	PresignDownload(ctx context.Context, key string, ttl time.Duration) (string, error)
	// Get — leximi i objektit nga vetë serveri (p.sh. imazhet publike kur s'ka CDN përpara).
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error)
	Delete(ctx context.Context, key string) error
}

var keyRe = regexp.MustCompile(`^[a-z0-9][a-z0-9/_.-]{3,255}$`)

// ValidKey — çelës i sigurt: pa "..", pa hapësira, vetëm shkronja të vogla/numra/-_./
func ValidKey(key string) bool {
	return keyRe.MatchString(key) && !strings.Contains(key, "..") && !strings.HasPrefix(key, "/")
}

// --- S3 --------------------------------------------------------------------------

type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

func NewS3(ctx context.Context, region, bucket string) (*S3, error) {
	if bucket == "" {
		return nil, errors.New("storage: S3_ASSETS_BUCKET mungon")
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg)
	return &S3{client: client, presign: s3.NewPresignClient(client), bucket: bucket}, nil
}

func (p *S3) PresignUpload(ctx context.Context, key, contentType string, sizeBytes int64, ttl time.Duration) (UploadTarget, error) {
	out, err := p.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(p.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(sizeBytes),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return UploadTarget{}, fmt.Errorf("storage: presign put: %w", err)
	}
	headers := map[string]string{"Content-Type": contentType, "Content-Length": fmt.Sprint(sizeBytes)}
	for k, v := range out.SignedHeader {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return UploadTarget{URL: out.URL, Method: out.Method, Headers: headers, ExpiresAt: time.Now().Add(ttl)}, nil
}

func (p *S3) PutBytes(ctx context.Context, key, contentType string, body []byte) error {
	_, err := p.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(p.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(int64(len(body))),
		Body:          bytes.NewReader(body),
	})
	if err != nil {
		return fmt.Errorf("storage: put: %w", err)
	}
	return nil
}

func (p *S3) Head(ctx context.Context, key string) (ObjectInfo, error) {
	out, err := p.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(p.bucket), Key: aws.String(key)})
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
			return ObjectInfo{}, ErrNotFound
		}
		return ObjectInfo{}, fmt.Errorf("storage: head: %w", err)
	}
	return ObjectInfo{Key: key, ContentType: aws.ToString(out.ContentType), SizeBytes: aws.ToInt64(out.ContentLength)}, nil
}

func (p *S3) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	out, err := p.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(p.bucket), Key: aws.String(key)})
	if err != nil {
		if strings.Contains(err.Error(), "NoSuchKey") || strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
			return nil, ObjectInfo{}, ErrNotFound
		}
		return nil, ObjectInfo{}, fmt.Errorf("storage: get: %w", err)
	}
	return out.Body, ObjectInfo{Key: key, ContentType: aws.ToString(out.ContentType), SizeBytes: aws.ToInt64(out.ContentLength)}, nil
}

func (p *S3) PresignDownload(ctx context.Context, key string, ttl time.Duration) (string, error) {
	out, err := p.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(p.bucket), Key: aws.String(key)}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("storage: presign get: %w", err)
	}
	return out.URL, nil
}

func (p *S3) Delete(ctx context.Context, key string) error {
	_, err := p.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(p.bucket), Key: aws.String(key)})
	return err
}

// --- DevFS (VETËM development) ----------------------------------------------------

// DevFS — objektet në një dosje lokale; ngarkimi kalon përmes `PUT {base}/api/v1/dev/uploads/{key}`
// (rrugë që API-ja e regjistron vetëm në development).
type DevFS struct {
	Dir     string
	BaseURL string
	mu      sync.Mutex
	pending map[string]pendingUpload
}

type pendingUpload struct {
	contentType string
	size        int64
	expires     time.Time
}

func NewDevFS(dir, baseURL string) (*DevFS, error) {
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "krejt-devfs")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return &DevFS{Dir: dir, BaseURL: strings.TrimRight(baseURL, "/"), pending: map[string]pendingUpload{}}, nil
}

func (d *DevFS) path(key string) string { return filepath.Join(d.Dir, filepath.FromSlash(key)) }

func (d *DevFS) PresignUpload(_ context.Context, key, contentType string, sizeBytes int64, ttl time.Duration) (UploadTarget, error) {
	d.mu.Lock()
	d.pending[key] = pendingUpload{contentType: contentType, size: sizeBytes, expires: time.Now().Add(ttl)}
	d.mu.Unlock()
	return UploadTarget{URL: d.BaseURL + "/api/v1/dev/uploads/" + key, Method: "PUT",
		Headers: map[string]string{"Content-Type": contentType, "Content-Length": fmt.Sprint(sizeBytes)}, ExpiresAt: time.Now().Add(ttl)}, nil
}

// Put — pranon ngarkimin (thirret nga endpoint-i dev); refuzon çelësa të panjohur/skaduar ose lloj/madhësi të ndryshme.
func (d *DevFS) Put(key, contentType string, body io.Reader) error {
	d.mu.Lock()
	p, ok := d.pending[key]
	d.mu.Unlock()
	if !ok || time.Now().After(p.expires) {
		return errors.New("storage: upload i panjohur ose i skaduar")
	}
	if contentType != p.contentType {
		return errors.New("storage: content-type ndryshe nga i premtuari")
	}
	if err := os.MkdirAll(filepath.Dir(d.path(key)), 0o700); err != nil {
		return err
	}
	f, err := os.Create(d.path(key))
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(body, p.size+1))
	if err != nil {
		return err
	}
	if n != p.size {
		_ = os.Remove(d.path(key))
		return errors.New("storage: madhësia ndryshe nga e premtuara")
	}
	if err := os.WriteFile(d.path(key)+".ct", []byte(contentType), 0o600); err != nil {
		return err
	}
	d.mu.Lock()
	delete(d.pending, key)
	d.mu.Unlock()
	return nil
}

// PutBytes — shkrim i drejtpërdrejtë nga serveri. Nuk kalon nga Put: ai verifikon një ngarkim të
// premtuar më parë me PresignUpload, ndërsa këtu objektin e prodhon vetë serveri.
func (d *DevFS) PutBytes(_ context.Context, key, contentType string, body []byte) error {
	if err := os.MkdirAll(filepath.Dir(d.path(key)), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(d.path(key), body, 0o600); err != nil {
		return err
	}
	return os.WriteFile(d.path(key)+".ct", []byte(contentType), 0o600)
}

func (d *DevFS) Head(_ context.Context, key string) (ObjectInfo, error) {
	st, err := os.Stat(d.path(key))
	if err != nil {
		return ObjectInfo{}, ErrNotFound
	}
	ct, _ := os.ReadFile(d.path(key) + ".ct")
	return ObjectInfo{Key: key, ContentType: string(ct), SizeBytes: st.Size()}, nil
}

func (d *DevFS) Get(_ context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	f, ct, err := d.Open(key)
	if err != nil {
		return nil, ObjectInfo{}, ErrNotFound
	}
	st, err := os.Stat(d.path(key))
	if err != nil {
		f.Close()
		return nil, ObjectInfo{}, ErrNotFound
	}
	return f, ObjectInfo{Key: key, ContentType: ct, SizeBytes: st.Size()}, nil
}

func (d *DevFS) PresignDownload(_ context.Context, key string, _ time.Duration) (string, error) {
	return d.BaseURL + "/api/v1/dev/uploads/" + url.PathEscape(key), nil
}

func (d *DevFS) Delete(_ context.Context, key string) error {
	_ = os.Remove(d.path(key) + ".ct")
	return os.Remove(d.path(key))
}

// Open — për endpoint-in dev GET.
func (d *DevFS) Open(key string) (io.ReadCloser, string, error) {
	ct, _ := os.ReadFile(d.path(key) + ".ct")
	f, err := os.Open(d.path(key))
	if err != nil {
		return nil, "", ErrNotFound
	}
	return f, string(ct), nil
}

// NewFromEnv — STORAGE_PROVIDER: s3 (parazgjedhje) | devfs (vetëm development).
func NewFromEnv(ctx context.Context, env, provider, region, bucket, devDir, devBaseURL string, log *slog.Logger) (Provider, error) {
	switch provider {
	case "s3", "":
		return NewS3(ctx, region, bucket)
	case "devfs":
		if env != "development" {
			return nil, fmt.Errorf("storage: devfs lejohet vetëm në development (APP_ENV=%s)", env)
		}
		log.Warn("DEV ONLY — STORAGE_PROVIDER=devfs: objektet ruhen në disk lokal", "dir", devDir)
		return NewDevFS(devDir, devBaseURL)
	default:
		return nil, fmt.Errorf("storage: ofrues i panjohur %q", provider)
	}
}
