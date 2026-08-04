package awaiting

import (
	"context"
	"sync"
)


// MPSC
type RefreshingValue[T any] struct {
	mu    sync.Mutex
	value T

	Await chan struct{}
	once  sync.Once
}

func NewRefreshingValue[T any]() *RefreshingValue[T] {
	return &RefreshingValue[T]{
		Await: make(chan struct{}),
	}
}

func (r *RefreshingValue[T]) AwaitNew() T {
	<-r.Await

	return r.Get()
}

func (r *RefreshingValue[T]) AwaitNewCtx(ctx context.Context) (T, error) {
	select {
	case <-r.Await:
		return r.Get(), nil
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	}
}

func (r *RefreshingValue[T]) Get() T {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.Await = make(chan struct{})
	r.once = sync.Once{}

	return r.value
}

func (r *RefreshingValue[T]) Store(value T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.value = value
	r.once.Do(func() {
		close(r.Await)
	})
}

func Refresher[T any](ctx context.Context, ch <-chan T, refresh func(value T)) {
	for {
		select {
		case <-ctx.Done():
			return
		case value := <-ch:
			refresh(value)
		}
	}
}
