package logging

import (
	"context"
	"sync"
	"testing"
)

func TestConcurrent(t *testing.T) {
	err := Init("logger_test", "snapshot", "info", "logfmt")
	if err != nil {
		t.Fatalf(err.Error())
	}

	t.Run("samectxhierarchy", func(t *testing.T) {
		ctx := NewContext(context.Background(), "hello", "world")
		numConcurrent := 64

		logger := WithContext(ctx)
		logger.Infof("before go-routines")

		var wg sync.WaitGroup
		wg.Add(numConcurrent)
		for i := 0; i < numConcurrent; i++ {
			go func(n int, ctx context.Context) {
				defer wg.Done()

				f1ctx := NewContext(ctx, "fval", n)
				l2 := WithContext(f1ctx)
				l2.Infof("from f%d", n)

				l2.Sync()
			}(i, ctx)
		}
		wg.Wait()

		logger = WithContext(ctx)
		logger.Infof("after go-routines")
	})
}
