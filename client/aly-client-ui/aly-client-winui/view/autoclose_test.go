// +build windows

package view

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestAutoCloseFires(t *testing.T) {
	var fired int32
	ac := ScheduleAutoClose(50*time.Millisecond, func() {
		atomic.AddInt32(&fired, 1)
	})
	defer ac.Cancel()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&fired) == 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("auto close did not fire within 3s")
}

func TestAutoCloseCancelPreventsFiring(t *testing.T) {
	var fired int32
	ac := ScheduleAutoClose(120*time.Millisecond, func() {
		atomic.AddInt32(&fired, 1)
	})
	ac.Cancel()

	time.Sleep(400 * time.Millisecond)
	if atomic.LoadInt32(&fired) != 0 {
		t.Fatal("auto close fired after Cancel")
	}
}

func TestAutoCloseCancelIdempotent(t *testing.T) {
	ac := ScheduleAutoClose(100*time.Millisecond, func() {})
	ac.Cancel()
	ac.Cancel()
	ac.Cancel()
	// 不应 panic（nil timer / 重复 Stop 都安全）
}

func TestAutoCloseNilSafe(t *testing.T) {
	var ac *AutoClose
	ac.Cancel() // nil 接收者不得 panic
}

func TestAutoCloseFiresOnlyOnce(t *testing.T) {
	var n int32
	ac := ScheduleAutoClose(30*time.Millisecond, func() {
		atomic.AddInt32(&n, 1)
	})
	defer ac.Cancel()

	time.Sleep(300 * time.Millisecond)
	if got := atomic.LoadInt32(&n); got != 1 {
		t.Fatalf("handler should run exactly once, ran %d times", got)
	}
}
