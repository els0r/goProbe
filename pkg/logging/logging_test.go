package logging

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// testLogger captures the log output from slog and logs it via the testing.T object,
// resulting in log lines only written if test is run with verbose
type testLogger struct {
	t *testing.T
}

func (tl *testLogger) Write(data []byte) (n int, err error) {
	tl.t.Log(strings.TrimRight(string(data), string('\n')))
	return n, err
}

func TestLogConcurrent(t *testing.T) {
	err := Init("snapshot", "info", "logfmt", WithOutput(&testLogger{t}))
	if err != nil {
		t.Fatalf(err.Error())
	}

	t.Run("samectxhierarchy", func(t *testing.T) {
		ctx := NewContext(context.Background(), "hello", "world")
		numConcurrent := 64

		logger := WithContext(ctx)
		logger.Info("before go-routines")

		var wg sync.WaitGroup
		wg.Add(numConcurrent)
		for i := 0; i < numConcurrent; i++ {
			go func(n int, ctx context.Context) {
				defer wg.Done()

				f1ctx := NewContext(ctx, "fval", n)
				l2 := WithContext(f1ctx)
				l2.Info(fmt.Sprintf("from f%d", n))
			}(i, ctx)
		}
		wg.Wait()

		logger = WithContext(ctx)
		logger.Info("after go-routines")
	})
}
