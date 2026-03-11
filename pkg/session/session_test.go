package session

import (
	"context"
	"testing"
	"time"
)

func TestMemorySession_SetGetDelete(t *testing.T) {
	s := NewMemorySession()
	ctx := context.Background()

	if err := s.Set(ctx, "k1", "hello", 10*time.Second); err != nil {
		t.Fatal(err)
	}

	val, err := s.Get(ctx, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if val != `"hello"` {
		t.Errorf("expected %q, got %q", `"hello"`, val)
	}

	if err := s.Delete(ctx, "k1"); err != nil {
		t.Fatal(err)
	}

	val, err = s.Get(ctx, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Errorf("expected empty after delete, got %q", val)
	}
}

func TestMemorySession_Expiry(t *testing.T) {
	s := NewMemorySession()
	ctx := context.Background()

	if err := s.Set(ctx, "k2", "data", 1*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Millisecond)

	val, err := s.Get(ctx, "k2")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Errorf("expected empty after expiry, got %q", val)
	}
}

func TestMemorySession_SetOverwrite(t *testing.T) {
	s := NewMemorySession()
	ctx := context.Background()

	s.Set(ctx, "k3", "v1", 10*time.Second)
	s.Set(ctx, "k3", "v2", 10*time.Second)

	val, _ := s.Get(ctx, "k3")
	if val != `"v2"` {
		t.Errorf("expected %q, got %q", `"v2"`, val)
	}
}

func TestMemorySession_StructValue(t *testing.T) {
	s := NewMemorySession()
	ctx := context.Background()

	data := map[string]string{"user": "alice", "role": "admin"}
	if err := s.Set(ctx, "k4", data, 10*time.Second); err != nil {
		t.Fatal(err)
	}

	val, err := s.Get(ctx, "k4")
	if err != nil {
		t.Fatal(err)
	}
	if val == "" {
		t.Error("expected non-empty value for struct")
	}
}
