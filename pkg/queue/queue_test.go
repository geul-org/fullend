package queue

import (
	"context"
	"sync"
	"testing"
)

func resetQueue() {
	mu.Lock()
	inited = false
	handlers = nil
	backend = ""
	db = nil
	cancel = nil
	done = nil
	mu.Unlock()
}

func TestPublishSubscribe(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	if err := Init(ctx, "memory", nil); err != nil {
		t.Fatal(err)
	}
	defer Close()

	var got string
	Subscribe("test.topic", func(_ context.Context, msg []byte) error {
		got = string(msg)
		return nil
	})

	if err := Publish(ctx, "test.topic", map[string]string{"hello": "world"}); err != nil {
		t.Fatal(err)
	}

	want := `{"hello":"world"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPublishNoSubscriber(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	if err := Init(ctx, "memory", nil); err != nil {
		t.Fatal(err)
	}
	defer Close()

	if err := Publish(ctx, "no.subscriber", "payload"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestMultipleSubscribers(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	if err := Init(ctx, "memory", nil); err != nil {
		t.Fatal(err)
	}
	defer Close()

	var muResult sync.Mutex
	results := make(map[string]string)

	Subscribe("topic.a", func(_ context.Context, msg []byte) error {
		muResult.Lock()
		results["a"] = string(msg)
		muResult.Unlock()
		return nil
	})

	Subscribe("topic.b", func(_ context.Context, msg []byte) error {
		muResult.Lock()
		results["b"] = string(msg)
		muResult.Unlock()
		return nil
	})

	if err := Publish(ctx, "topic.a", "alpha"); err != nil {
		t.Fatal(err)
	}
	if err := Publish(ctx, "topic.b", "beta"); err != nil {
		t.Fatal(err)
	}

	muResult.Lock()
	defer muResult.Unlock()

	if results["a"] != `"alpha"` {
		t.Errorf("topic.a: got %q, want %q", results["a"], `"alpha"`)
	}
	if results["b"] != `"beta"` {
		t.Errorf("topic.b: got %q, want %q", results["b"], `"beta"`)
	}
}

func TestWithDelay(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	if err := Init(ctx, "memory", nil); err != nil {
		t.Fatal(err)
	}
	defer Close()

	called := false
	Subscribe("delayed", func(_ context.Context, msg []byte) error {
		called = true
		return nil
	})

	if err := Publish(ctx, "delayed", "data", WithDelay(30)); err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Error("handler should be called even with delay on memory backend")
	}
}

func TestWithPriority(t *testing.T) {
	resetQueue()
	ctx := context.Background()

	if err := Init(ctx, "memory", nil); err != nil {
		t.Fatal(err)
	}
	defer Close()

	called := false
	Subscribe("prio", func(_ context.Context, msg []byte) error {
		called = true
		return nil
	})

	if err := Publish(ctx, "prio", "data", WithPriority("high")); err != nil {
		t.Fatal(err)
	}

	if !called {
		t.Error("handler should be called with priority option on memory backend")
	}
}

func TestPublishBeforeInit(t *testing.T) {
	resetQueue()

	err := Publish(context.Background(), "topic", "data")
	if err != ErrNotInitialized {
		t.Errorf("got %v, want %v", err, ErrNotInitialized)
	}
}
