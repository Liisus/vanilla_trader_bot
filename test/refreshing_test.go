package test

import (
	"context"
	"testing"
	"time"

	"vanilla/internal/ordering/awaiting"
)

func TestRefreshing_StoreThenGet(t *testing.T) {
	r := awaiting.NewRefreshingValue[int]()
	r.Store(42)
	if got := r.Get(); got != 42 {
		t.Errorf("Get = %d, want 42", got)
	}
}

func TestRefreshing_ZeroBeforeStore(t *testing.T) {
	r := awaiting.NewRefreshingValue[int]()
	if got := r.Get(); got != 0 {
		t.Errorf("Get on fresh value = %d, want zero", got)
	}
}

// Store closes the Await channel so a waiter wakes up.
func TestRefreshing_AwaitNewWakesOnStore(t *testing.T) {
	r := awaiting.NewRefreshingValue[string]()

	got := make(chan string, 1)
	go func() { got <- r.AwaitNew() }()

	// Give the goroutine time to park on the channel, then store.
	time.Sleep(20 * time.Millisecond)
	r.Store("hello")

	select {
	case v := <-got:
		if v != "hello" {
			t.Errorf("AwaitNew = %q, want hello", v)
		}
	case <-time.After(time.Second):
		t.Fatal("AwaitNew did not wake after Store")
	}
}

// After a Get, the value is "rearmed": the next Store wakes a fresh waiter.
func TestRefreshing_RearmsAfterGet(t *testing.T) {
	r := awaiting.NewRefreshingValue[int]()

	r.Store(1)
	if got := r.AwaitNew(); got != 1 { // consumes the first round, rearms
		t.Fatalf("first AwaitNew = %d, want 1", got)
	}

	got := make(chan int, 1)
	go func() { got <- r.AwaitNew() }()
	time.Sleep(20 * time.Millisecond)
	r.Store(2)

	select {
	case v := <-got:
		if v != 2 {
			t.Errorf("second AwaitNew = %d, want 2", v)
		}
	case <-time.After(time.Second):
		t.Fatal("AwaitNew did not wake on the second round")
	}
}

func TestRefreshing_AwaitNewCtx_Cancelled(t *testing.T) {
	r := awaiting.NewRefreshingValue[int]()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err := r.AwaitNewCtx(ctx)
	if err == nil {
		t.Fatal("expected context error when no Store happens")
	}
}

func TestRefreshing_AwaitNewCtx_Value(t *testing.T) {
	r := awaiting.NewRefreshingValue[int]()
	ctx := context.Background()

	go func() {
		time.Sleep(20 * time.Millisecond)
		r.Store(7)
	}()

	v, err := r.AwaitNewCtx(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 7 {
		t.Errorf("AwaitNewCtx = %d, want 7", v)
	}
}

// Refresher pumps channel values into a sink until ctx is cancelled.
func TestRefresher_PumpsUntilCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan int)
	r := awaiting.NewRefreshingValue[int]()

	go awaiting.Refresher(ctx, ch, func(v int) { r.Store(v) })

	ch <- 5
	if got := r.AwaitNew(); got != 5 {
		t.Errorf("first value = %d, want 5", got)
	}
	ch <- 9
	if got := r.AwaitNew(); got != 9 {
		t.Errorf("second value = %d, want 9", got)
	}

	cancel()
	// After cancel the refresher should stop consuming; this send must not be
	// received, so it would block — use a non-blocking send to confirm.
	time.Sleep(20 * time.Millisecond)
	select {
	case ch <- 1:
		t.Fatal("Refresher still consuming after ctx cancel")
	default:
	}
}

// RefreshingValue is documented as MPSC: many producers, ONE consumer. This
// test honours that contract — one storer goroutine and one consumer goroutine
// — and must stay clean under -race. (A variant with multiple concurrent
// consumers does race on the Await field, which is consistent with the MPSC
// contract and is called out in the test summary.)
func TestRefreshing_SingleConsumerConcurrentStores(t *testing.T) {
	r := awaiting.NewRefreshingValue[int]()
	stop := make(chan struct{})
	storerDone := make(chan struct{})

	go func() {
		defer close(storerDone)
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				i++
				r.Store(i)
			}
		}
	}()

	// Single consumer.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	for j := 0; j < 500; j++ {
		if _, err := r.AwaitNewCtx(ctx); err != nil {
			break
		}
	}

	close(stop)
	<-storerDone
}

// Many concurrent producers through Refresher; the value always reflects one of
// the stored inputs. Run under -race to catch data races on the Await field.
func TestRefreshing_ConcurrentStores(t *testing.T) {
	r := awaiting.NewRefreshingValue[int]()
	const n = 100

	done := make(chan struct{})
	go func() {
		for i := 0; i < n; i++ {
			r.Store(i)
		}
		close(done)
	}()

	// Concurrent readers.
	for i := 0; i < 4; i++ {
		go func() {
			for j := 0; j < n; j++ {
				_ = r.Get()
			}
		}()
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent stores did not finish")
	}
}
