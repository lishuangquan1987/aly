package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type layout struct {
	update     string
	main       string
	ui         string
	updater    string
	clientJSON string
	sharedJSON string
}

type clientConfig struct {
	MainExeRelativePath string `json:"main_exe_relative_path"`
}

type sharedConfig struct {
	ProjectName string `json:"project_name"`
}

func executableDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe), nil
}

func loadLayout() (layout, error) {
	update, err := executableDir()
	if err != nil {
		return layout{}, err
	}
	clientJSON := filepath.Join(update, "client.json")
	b, err := ioutil.ReadFile(clientJSON)
	if err != nil {
		return layout{}, fmt.Errorf("read client.json: %v", err)
	}
	var cc clientConfig
	if err := json.Unmarshal(b, &cc); err != nil {
		return layout{}, fmt.Errorf("parse client.json: %v", err)
	}
	if cc.MainExeRelativePath == "" {
		return layout{}, fmt.Errorf("client.json: main_exe_relative_path is empty")
	}
	// 同一份 client.json 可能由 Windows 打包工具写出；Linux 按 POSIX 分隔符解释。
	rel := strings.ReplaceAll(cc.MainExeRelativePath, "\\", "/")
	main := filepath.Dir(filepath.Join(update, rel))
	return layout{
		update:     update,
		main:       main,
		ui:         filepath.Join(update, "aly-client-ui"),
		updater:    filepath.Join(update, "aly-client"),
		clientJSON: clientJSON,
		sharedJSON: filepath.Join(main, ".updator", "shared.json"),
	}, nil
}

func resolveProjectName(l layout, override string) string {
	if override != "" {
		return override
	}
	b, err := ioutil.ReadFile(l.sharedJSON)
	if err == nil {
		var sc sharedConfig
		if json.Unmarshal(b, &sc) == nil && sc.ProjectName != "" {
			return sc.ProjectName
		}
	}
	return "aly-client-ui"
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func sameFile(a, b string) (bool, error) {
	ha, err := sha256File(a)
	if err != nil {
		return false, err
	}
	hb, err := sha256File(b)
	if err != nil {
		return false, err
	}
	return ha == hb, nil
}

// copyVerify 复制 src→dst（0755），可选保留可执行位，落盘 fsync 后用 SHA256 校验。
func copyVerify(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fi.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	want, err := sha256File(src)
	if err != nil {
		return err
	}
	got, err := sha256File(dst)
	if err != nil {
		return err
	}
	if want != got {
		return fmt.Errorf("copy verify failed: sha256 %s != %s", got, want)
	}
	return nil
}

func nextAsidePath(target, suffix string) string {
	base := target + suffix
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base
	}
	for i := 1; i < 1000; i++ {
		p := fmt.Sprintf("%s.%d", base, i)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			return p
		}
	}
	return fmt.Sprintf("%s.%d", base, time.Now().UnixNano())
}

// cleanArtifacts 清理 target 的 .new 与 .old*（target 存在时）；target 缺失时保留 .old。
func cleanArtifacts(target string) {
	if _, err := os.Stat(target); err != nil {
		return
	}
	_ = os.Remove(target + ".new")
	if matches, err := filepath.Glob(target + ".old*"); err == nil {
		for _, p := range matches {
			_ = os.RemoveAll(p)
		}
	}
}

// replaceFromFile 直接复制并原子替换 target（不含 .old 备份；仅兜底用）。
func replaceFromFile(src, target string) error {
	tmp := target + ".tmp"
	if err := copyVerify(src, tmp); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
