// +build windows

package app

import "syscall"

// kernel32DLL 是共享的 kernel32 句柄，供本包各处按需取用过程地址。
var kernel32DLL = syscall.NewLazyDLL("kernel32.dll")
