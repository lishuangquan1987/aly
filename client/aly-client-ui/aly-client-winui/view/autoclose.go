// +build windows

package view

import (
	"sync"
	"time"
)

// AutoClose 是一个只触发一次的延时回调，可提前取消。
//
// 从窗口代码中抽出，便于脱离 GUI 单元测试。
// 典型用途：-mode check 下「无更新」提示显示 5s 后自动关闭窗口。
type AutoClose struct {
	t    *time.Timer
	once sync.Once
}

// ScheduleAutoClose 在 d 之后异步调用 onFire（在调用方的协程上下文执行，
// 因此 UI 场景下 onFire 内部必须自行回到 UI 线程，例如 walk.Synchronize）。
func ScheduleAutoClose(d time.Duration, onFire func()) *AutoClose {
	ac := &AutoClose{}
	ac.t = time.AfterFunc(d, func() {
		ac.once.Do(onFire)
	})
	return ac
}

// Cancel 取消回调。可重复调用；对 nil 接收者安全。
func (a *AutoClose) Cancel() {
	if a == nil || a.t == nil {
		return
	}
	a.t.Stop()
}
