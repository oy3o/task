package task

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafely(t *testing.T) {
	t.Run("Should catch panic and call handler", func(t *testing.T) {
		var wg sync.WaitGroup
		var recovered any

		handler := func(ctx context.Context, p any) {
			recovered = p
			wg.Done()
		}

		wg.Add(1)
		// 模拟一个 Panic
		Safely(context.Background(), func(_ context.Context) {
			panic("boom")
		}, handler)

		wg.Wait()
		assert.Equal(t, "boom", recovered)
	})

	t.Run("Should execute normal function without panic", func(t *testing.T) {
		executed := false
		Safely(context.Background(), func(_ context.Context) {
			executed = true
		}, nil)
		assert.True(t, executed)
	})
}

func TestGo(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	Go(context.Background(), func(ctx context.Context) {
		defer wg.Done()
		// 正常执行
	}, nil)

	wg.Wait()
}
