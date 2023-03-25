package capture

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestLowTrafficDeadlock(t *testing.T) {
	testDeadlock(t, 0)
	testDeadlock(t, 1)
	testDeadlock(t, 10)
}

func TestHighTrafficDeadlock(t *testing.T) {
	testDeadlock(t, -1)
}

func testDeadlock(t *testing.T, maxPkts int) {
	ctx := context.Background()
	mockSrc := newMockCaptureSource(maxPkts)
	mockC := &Capture{
		iface:         "none",
		mutex:         sync.Mutex{},
		cmdChan:       make(chan captureCommand),
		captureErrors: make(chan error),
		lastRotationStats: Stats{
			CaptureStats: &CaptureStats{},
		},
		rotationState: newRotationState(),
		flowLog:       NewFlowLog(),
		errMap:        make(map[string]int),
		ctx:           ctx,
		captureHandle: mockSrc,
	}

	start := time.Now()
	time.AfterFunc(100*time.Millisecond, func() {
		for i := 0; i < 10; i++ {
			mockC.rotationState.request <- struct{}{}
			if err := mockC.captureHandle.Unblock(); err != nil {
				panic(err)
			}

			<-mockC.rotationState.confirm
			mockC.flowLog.Rotate()
			mockC.rotationState.done <- struct{}{}

			time.Sleep(10 * time.Millisecond)
		}

		mockSrc.Close()
	})

	mockC.process()

	if time.Since(start) > 2*time.Second {
		t.Fatalf("potential deadlock situation on rotation logic")
	}
}

func TestMockPacketCapturePerformance(t *testing.T) {

	if testing.Short() {
		t.SkipNow()
	}

	ctx := context.Background()
	mockSrc := newMockCaptureSource(-1)
	mockC := &Capture{
		iface:         "none",
		mutex:         sync.Mutex{},
		cmdChan:       make(chan captureCommand),
		captureErrors: make(chan error),
		lastRotationStats: Stats{
			CaptureStats: &CaptureStats{},
		},
		rotationState: newRotationState(),
		flowLog:       NewFlowLog(),
		errMap:        make(map[string]int),
		ctx:           ctx,
		captureHandle: mockSrc,
	}

	runtime := 10 * time.Second
	time.AfterFunc(runtime, func() {
		mockSrc.Close()
	})

	mockC.process()
	for _, v := range mockC.flowLog.flowMap {
		fmt.Printf("Packets processed after %v: %d (%v/pkt)\n", runtime, v.packetsRcvd, runtime/time.Duration(v.packetsRcvd))
	}
}
