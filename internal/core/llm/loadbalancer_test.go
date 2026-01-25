package llm

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestRoundRobinLoadBalancer_Empty(t *testing.T) {
	t.Parallel()

	lb := NewLoadBalancer("round_robin", newTestLogger())
	if _, err := lb.SelectBackend(context.Background(), &LLMRequest{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRoundRobinLoadBalancer_SingleBackend(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	lb := NewLoadBalancer("round_robin", logger)

	b1 := &mockBackend{name: "b1"}
	lb.AddBackend(b1)

	for i := 0; i < 5; i++ {
		b, err := lb.SelectBackend(context.Background(), &LLMRequest{})
		if err != nil {
			t.Fatalf("SelectBackend: %v", err)
		}
		if b.GetName() != "b1" {
			t.Fatalf("got=%s want b1", b.GetName())
		}
	}
}

func TestRoundRobinLoadBalancer_MultiBackendOrder(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	lb := NewLoadBalancer("round_robin", logger)

	b1 := &mockBackend{name: "b1"}
	b2 := &mockBackend{name: "b2"}
	b3 := &mockBackend{name: "b3"}
	lb.AddBackend(b1)
	lb.AddBackend(b2)
	lb.AddBackend(b3)

	want := []string{"b1", "b2", "b3", "b1"}
	for i := range want {
		b, err := lb.SelectBackend(context.Background(), &LLMRequest{})
		if err != nil {
			t.Fatalf("SelectBackend: %v", err)
		}
		if b.GetName() != want[i] {
			t.Fatalf("i=%d got=%s want %s", i, b.GetName(), want[i])
		}
	}
}

func TestRoundRobinLoadBalancer_ConcurrentSelect(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	lb := NewLoadBalancer("round_robin", logger)

	lb.AddBackend(&mockBackend{name: "b1"})
	lb.AddBackend(&mockBackend{name: "b2"})
	lb.AddBackend(&mockBackend{name: "b3"})

	var wg sync.WaitGroup
	errCh := make(chan error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := lb.SelectBackend(context.Background(), &LLMRequest{})
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("SelectBackend: %v", err)
		}
	}
}

func TestRoundRobinLoadBalancer_Report(t *testing.T) {
	t.Parallel()

	logger := newTestLogger()
	lb := NewLoadBalancer("round_robin", logger)

	lb.ReportSuccess("b1", 0)
	lb.ReportError("b1", errors.New("x"))
}
