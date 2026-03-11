package cache

import (
	"context"
	"testing"
	"time"
)

func TestMemoryCache_SetGetDelete(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	if err := c.Set(ctx, "k1", "hello", 10*time.Second); err != nil {
		t.Fatal(err)
	}

	val, err := c.Get(ctx, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if val != `"hello"` {
		t.Errorf("expected %q, got %q", `"hello"`, val)
	}

	if err := c.Delete(ctx, "k1"); err != nil {
		t.Fatal(err)
	}

	val, err = c.Get(ctx, "k1")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Errorf("expected empty after delete, got %q", val)
	}
}

func TestMemoryCache_Expiry(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	if err := c.Set(ctx, "k2", "data", 1*time.Millisecond); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Millisecond)

	val, err := c.Get(ctx, "k2")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Errorf("expected empty after expiry, got %q", val)
	}
}

func TestMemoryCache_SetOverwrite(t *testing.T) {
	c := NewMemoryCache()
	ctx := context.Background()

	c.Set(ctx, "k3", "v1", 10*time.Second)
	c.Set(ctx, "k3", "v2", 10*time.Second)

	val, _ := c.Get(ctx, "k3")
	if val != `"v2"` {
		t.Errorf("expected %q, got %q", `"v2"`, val)
	}
}
