package logging

import (
	"context"
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
				l2.Infof("f%d", n)
			}(i, ctx)
		}
		wg.Wait()

		logger = WithContext(ctx)
		logger.Infof("after %d go-routines", numConcurrent)
	})
}

type mockExiter struct {
	t *testing.T
}

func (m mockExiter) Exit(code int) {
	m.t.Logf("mocking exit with code: %d", code)
}

type mockPanicker struct {
	t *testing.T
}

func (m mockPanicker) Panic(msg string) {
	m.t.Logf("mocking panic with message: %s", msg)
}

func TestCustomLogMessages(t *testing.T) {
	err := Init("snapshot", "info", "logfmt", WithOutput(&testLogger{t}))
	if err != nil {
		t.Fatalf(err.Error())
	}

	t.Run("testformatting", func(t *testing.T) {
		logger := WithContext(nil)
		// the next two lines shouldn't show up due to level "info" in init
		logger.Debug("f", "hello", "world")
		logger.Debugf("f%d", LevelDebug)

		logger.Info("f", "level_num", LevelInfo)
		logger.Infof("f%d", LevelInfo)
		logger.Warn("f", "level_num", LevelWarn)
		logger.Warnf("f%d", LevelWarn)
		logger.Error("f", LevelError, "level_num", LevelError)
		logger.Errorf("f%d", LevelError)
	})

	t.Run("panic", func(t *testing.T) {
		logger := WithContext(nil).panicker(mockPanicker{t})
		logger.Panic("this my dearest friends, is where I leave you", "friends_left", 42)
		logger.Panicf("this my dearest friends, is where I leave %s", "you")
	})

	t.Run("fatal", func(t *testing.T) {
		logger := WithContext(nil).exiter(mockExiter{t})
		logger.Fatal("this my dearest friends, is where I leave you", "friends_left", 24)
		logger.Fatalf("this my dearest friends, is where I leave %s", "you")
	})
}
