// +build windows

package app

import (
	"syscall"
	"unsafe"
)

const (
	errorAlreadyExists = 183
	// 不加 Global\ 前缀：默认走**会话**命名空间。
	// Global\ 在 XP 多用户/快速用户切换下会因默认 ACL 而失败，
	// 而 UI 本来就是每用户一个实例。
	muName = `AlyClientUI_SingleInstance`

	// evtName 是「手动检查请求」命名事件（auto-reset）。
	// 第二实例（用户点了主程序的「检查更新」）拿不到互斥体时通过它
	// 通知第一实例「有人想看检查结果」，第一实例据此把静默流程升级为
	// 交互式——否则 auto 模式存活期间用户点了按钮什么都不会出现
	//（DEVELOPMENT_PLAN §8/M5 承诺的转发机制，本次落地）。
	evtName = `AlyClientUI_CheckRequested`

	waitTimeoutMs = 200 // WaitForSingleObject 轮询间隔（也为 stop 留出口）
)

var (
	procCreateMutex         = kernel32DLL.NewProc("CreateMutexW")
	procCloseHandle         = kernel32DLL.NewProc("CloseHandle")
	procCreateEvent         = kernel32DLL.NewProc("CreateEventW")
	procOpenEvent           = kernel32DLL.NewProc("OpenEventW")
	procSetEvent            = kernel32DLL.NewProc("SetEvent")
	procWaitForSingleObject = kernel32DLL.NewProc("WaitForSingleObject")
)

// eventModifyState 是 SetEvent 需要的事件访问权（EVENT_MODIFY_STATE）。
const eventModifyState = 0x0002

// AcquireSingleInstance 用命名互斥体实现单实例。
//
// 返回 (是否首个实例, 释放函数)。拿不到互斥体（极端情况）时按「首个实例」
// 处理并放行，避免因为系统调用失败而让 UI 完全起不来。
func AcquireSingleInstance() (bool, func()) {
	name, err := syscall.UTF16PtrFromString(muName)
	if err != nil {
		return true, func() {}
	}

	h, _, callErr := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return true, func() {}
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno == errorAlreadyExists {
		procCloseHandle.Call(h)
		return false, func() {}
	}

	released := false
	return true, func() {
		if released {
			return
		}
		released = true
		procCloseHandle.Call(h)
	}
}

// NotifyCheckRequested 向第一实例发「手动检查请求」信号。
// 返回是否成功送达（无第一实例时 OpenEventW 失败，返回 false——
// 调用方据此知道自己是唯一的，走正常路径）。
func NotifyCheckRequested() bool {
	name, err := syscall.UTF16PtrFromString(evtName)
	if err != nil {
		return false
	}
	// EVENT_MODIFY_STATE：SetEvent 所需的最小权限。
	h, _, _ := procOpenEvent.Call(eventModifyState, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return false
	}
	defer procCloseHandle.Call(h)
	r, _, _ := procSetEvent.Call(h)
	return r != 0
}

// ListenCheckRequested 创建「手动检查请求」事件并返回监听通道。
//
// 返回的通道在信号到达或 stop 被调用后关闭；通道关闭后应停止 select。
// 事件为 auto-reset：第一实例正在处理上一个请求期间到来的新信号
// 由内核暂存一次，不会丢（连点场景合并为一次处理，正合适）。
func ListenCheckRequested() (<-chan struct{}, func()) {
	name, err := syscall.UTF16PtrFromString(evtName)
	if err != nil {
		ch := make(chan struct{})
		return ch, func() {}
	}

	// 手动 reset=false → auto-reset 事件：WaitForSingleObject 命中后自动复位。
	// 初始状态 false：先启动监听，信号后到才有效。
	h, _, _ := procCreateEvent.Call(0, 0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		ch := make(chan struct{})
		return ch, func() {}
	}

	ch := make(chan struct{}, 1)
	stopCh := make(chan struct{})
	go func() {
		defer close(ch)
		defer procCloseHandle.Call(h)
		for {
			// 分片等待：stop 到来时最多 waitTimeoutMs 后退出，不留僵尸 goroutine。
			r, _, _ := procWaitForSingleObject.Call(h, waitTimeoutMs)
			select {
			case <-stopCh:
				return
			default:
			}
			if r == 0 { // WAIT_OBJECT_0
				select {
				case ch <- struct{}{}:
				default: // 上一信号还没被消费：合并（连点去抖）
				}
				continue
			}
			// r != 0（超时 0x102 等）：继续轮询，直到 stop 或命中
		}
	}()
	return ch, func() { close(stopCh) }
}
