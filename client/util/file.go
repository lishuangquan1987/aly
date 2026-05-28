package util

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FileMD5 计算文件的 MD5 哈希值
func FileMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// FileSHA256 计算文件的 SHA-256 哈希值
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CopyFile 复制单个文件，overwrite 控制是否覆盖目标文件
func CopyFile(src, dst string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(dst); err == nil {
			return nil
		}
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败 %s: %v", src, err)
	}
	defer srcFile.Close()

	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败 %s: %v", dstDir, err)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败 %s: %v", dst, err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("复制文件内容失败: %v", err)
	}

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return nil
	}
	dstFile.Chmod(srcInfo.Mode())

	return nil
}

// CopyDir 复制整个目录，overwrite 控制是否覆盖已存在的文件
func CopyDir(srcDir, dstDir string, overwrite bool) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dstDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		return CopyFile(path, dstPath, overwrite)
	})
}

// LocalFileMD5Map 获取本地目录下所有文件的 MD5 映射 (relativePath -> md5)
// 排除 update 目录
func LocalFileMD5Map(root string) (map[string]string, error) {
	result := make(map[string]string)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			relDir, _ := filepath.Rel(root, path)
			if relDir == "update" || hasPrefix(relDir, "update"+string(filepath.Separator)) || hasPrefix(relDir, "update/") {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if hasPrefix(relPath, "update"+string(filepath.Separator)) || hasPrefix(relPath, "update/") {
			return nil
		}

		md5, err := FileMD5(path)
		if err != nil {
			return fmt.Errorf("计算文件 MD5 失败 %s: %v", path, err)
		}

		result[relPath] = md5
		return nil
	})

	return result, err
}

// hasPrefix 检查字符串是否有指定前缀
func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// EnsureDir 确保目录存在
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// FileSize 获取文件大小
func FileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// CopyDirWithExclude copies all files from srcDir to dstDir,
// skipping files/folders for which shouldSkipFile or shouldSkipFolder returns true.
// The skip predicates receive paths relative to srcDir.
func CopyDirWithExclude(srcDir, dstDir string, shouldSkipFile func(relPath string) bool, shouldSkipFolder func(relPath string) bool) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Check if this directory should be skipped
			// Top-level "." is never skipped so we can still walk into subdirs
			if relPath != "." && shouldSkipFolder != nil && shouldSkipFolder(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if this file should be skipped
		if shouldSkipFile != nil && shouldSkipFile(relPath) {
			return nil
		}

		dstPath := filepath.Join(dstDir, relPath)
		// overwrite=false: don't overwrite files already in dstDir (e.g. newly downloaded files)
		return CopyFile(path, dstPath, false)
	})
}

// AppendToLog appends a line to a log file (creates it if needed).
// logDir is the directory where the log file is created.
func AppendToLog(logDir, filename, line string) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return
	}
	logPath := filepath.Join(logDir, filename)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line + "\n")
}
