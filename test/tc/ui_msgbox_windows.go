//go:build windows

package tc

// -mode check 的「无更新」分支会弹一个**阻塞式**模态框（walk.MsgBox），
// 必须点「确定」才会执行后续的 quit(w)。自动化里没有手，于是进程永不退出。
// 这里用 Win32 的 FindWindow + PostMessage 把那个 MessageBox 点掉，
// 使「进程最终是否干净退出」这条不变量仍可被断言。
//
// 只在 windows 构建下存在；非 windows 平台走同名的 no-op 实现。

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procFindWindowW  = user32.NewProc("FindWindowW")
	procPostMessageW = user32.NewProc("PostMessageW")
)

const (
	msgBoxClass = "#32770" // 标准对话框窗口类名（MessageBox / TaskDialog 共用）
	wmCommand   = 0x0111   // WM_COMMAND
	wmClose     = 0x0010   // WM_CLOSE
	idokCmd     = 1        // IDOK 按钮的控件 id
)

func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		return nil
	}
	return p
}

// dismissMessageBox 找到标题为 title 的标准对话框并点掉它（IDOK）。
// 返回是否真的点掉了一个窗口。
//
// 找不到就返回 false——调用方据此判断"这一轮压根没弹框"，
// 而不是把它当失败：例如 auto/silent 模式本来就不弹。
func dismissMessageBox(title string, wait time.Duration) bool {
	cls := utf16Ptr(msgBoxClass)
	if cls == nil {
		return false
	}
	cap := utf16Ptr(title)
	if cap == nil {
		return false
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(cls)),
			uintptr(unsafe.Pointer(cap)))
		if hwnd != 0 {
			// 先发 IDOK（点"确定"），再补一个 WM_CLOSE 兜底：
			// 某些主题下 MessageBox 不响应 WM_COMMAND/IDOK。
			procPostMessageW.Call(hwnd, wmCommand, idokCmd, 0)
			time.Sleep(80 * time.Millisecond)
			hwnd2, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(cls)),
				uintptr(unsafe.Pointer(cap)))
			if hwnd2 != 0 {
				procPostMessageW.Call(hwnd2, wmClose, 0, 0)
			}
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// messageBoxExists 判断此刻是否存在标题为 title 的标准对话框。
func messageBoxExists(title string) bool {
	cls := utf16Ptr(msgBoxClass)
	cap := utf16Ptr(title)
	if cls == nil || cap == nil {
		return false
	}
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(cls)),
		uintptr(unsafe.Pointer(cap)))
	return hwnd != 0
}