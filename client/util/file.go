package util

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// FileMD5 calculates the MD5 hash of a file.
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

// FileSHA256 calculates the SHA-256 hash of a file.
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

// CopyFile copies a single file. overwrite controls whether to overwrite the target file.
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

	srcInfo, statErr := srcFile.Stat()
	if statErr != nil {
		return fmt.Errorf("获取源文件信息失败 %s: %v", src, statErr)
	}
	// Chmod is not supported on Windows; ignore on unsupported platforms
	if chmodErr := dstFile.Chmod(srcInfo.Mode()); chmodErr != nil && runtime.GOOS != "windows" {
		return fmt.Errorf("chmod failed %s: %v", dst, chmodErr)
	}

	return nil
}

// CopyDir copies an entire directory. overwrite controls whether to overwrite existing files.
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

// LocalFileMD5Map returns a map of relativePath -> md5 for all files under root.
// The "update" directory is excluded. Keys use forward slashes for cross-platform consistency.
func LocalFileMD5Map(root string) (map[string]string, error) {
	result := make(map[string]string)

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		// Normalize to forward slashes for consistent map keys
		relPath = filepath.ToSlash(relPath)

		if info.IsDir() {
			if relPath == "update" || hasPrefix(relPath, "update/") {
				return filepath.SkipDir
			}
			return nil
		}

		if hasPrefix(relPath, "update/") {
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

// hasPrefix checks if a string has the specified prefix.
func hasPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}

// EnsureDir ensures a directory exists.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// FileSize returns the size of a file.
func FileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// CopyDirWithExclude copies all files from srcDir to dstDir,
// skipping files/folders for which shouldSkipFile or shouldSkipFolder returns true.
// The skip predicates receive paths relative to srcDir (OS-native separators).
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
			if relPath != "." && shouldSkipFolder != nil && shouldSkipFolder(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		if shouldSkipFile != nil && shouldSkipFile(relPath) {
			return nil
		}

		dstPath := filepath.Join(dstDir, relPath)
		return CopyFile(path, dstPath, false)
	})
}

// AppendToLog appends a line to a log file (creates it if needed).
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
