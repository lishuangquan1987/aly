// Package pathcfg 统一解析服务端的基础路径（可执行目录 / data 目录 / 数据库文件）。
//
// 背景（审查 A7）：exe 目录推导原先分散在 db.go、project_service.go、
// system_controller.go、file_download_controller.go 四处，且各处回退逻辑不一致——
// go run 调试时 aly.db 落工作目录、data/ 却落临时 exe 目录，同一进程出现两个
// "基础目录"。集中到本包后，所有路径只有一个真源，并给测试提供注入点。
package pathcfg

import (
	"log"
	"os"
	"path/filepath"
	"strings"
)

// baseDirOverride 测试注入的基础目录；生产环境恒为空。
var baseDirOverride string

// SetBaseDir 设置基础目录覆盖，仅供测试使用；传空串恢复默认推导。
func SetBaseDir(dir string) {
	baseDirOverride = dir
}

// BaseDir 返回服务端基础目录。
//
// 默认取可执行文件所在目录；仅当可执行文件位于 go 工具链的构建临时目录
// （路径中含 "go-build"，即 `go run` 的产物位置）时回退到当前工作目录，
// 否则调试时数据库/data 会落到临时目录、用户找不到。
//
// 注意：**不能**用 os.TempDir() 做判据——E2E 测试正是把 server.exe 建在
// %TEMP%\aly-tc-xxx\bin 下运行的，那时 exe 目录才是正确的 data 根目录。
func BaseDir() (string, error) {
	if baseDirOverride != "" {
		return baseDirOverride, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exeDir := filepath.Dir(exe)
	// go run 的产物位于 <TMPDIR>/go-build*，据此精确识别
	if strings.Contains(strings.ToLower(exeDir), "go-build") {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return wd, nil
	}
	return exeDir, nil
}

// DataDir 返回项目数据根目录（{BaseDir}/data）。
func DataDir() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "data"), nil
}

// DBPath 返回默认数据库文件路径（{BaseDir}/aly.db）。
func DBPath() (string, error) {
	base, err := BaseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "aly.db"), nil
}

// MustBaseDir 返回基础目录，失败时记录日志并返回工作目录（仅用于无法返回错误的
// 场景，如启动期日志）。调用方应优先使用返回 error 的版本。
func MustBaseDir() string {
	base, err := BaseDir()
	if err == nil {
		return base
	}
	log.Printf("pathcfg: failed to resolve base dir, fallback to working directory: %v", err)
	if wd, err2 := os.Getwd(); err2 == nil {
		return wd
	}
	return "."
}