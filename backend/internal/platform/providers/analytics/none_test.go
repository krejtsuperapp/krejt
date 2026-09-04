package analytics

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// Të mos matësh asgjë është zgjedhje e ligjshme; devlog-u, që shtiret se dërgon, mbetet i ndaluar.
func TestNoneAllowedInProduction(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	p, err := NewFromEnv("production", "none", "", "", log)
	if err != nil {
		t.Fatalf("none duhet lejuar në prodhim: %v", err)
	}
	p.Capture(Event{Event: "test"})
	if err := p.Close(context.Background()); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := NewFromEnv("production", "devlog", "", "", log); err == nil {
		t.Fatal("devlog nuk lejohet në prodhim")
	}
}
