package utils

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

func GetFileMD5(filename string) (string, error) {
	f, err := os.Open(filename) //打开文件
	if err != nil {
		return "", err
	}
	defer f.Close()

	md5Handle := md5.New()         //创建 md5 句柄
	_, err = io.Copy(md5Handle, f) //将文件内容拷贝到 md5 句柄中
	if err != nil {
		return "", err
	}
	md := md5Handle.Sum(nil) //计算 MD5 值，返回 []byte

	md5str := fmt.Sprintf("%x", md) //将 []byte 转为 string
	return md5str, nil
}

func GetFileSHA256(filename string) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sha256Handle := sha256.New()
	_, err = io.Copy(sha256Handle, f)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256Handle.Sum(nil)), nil
}

// GetFileMD5AndSHA256 单遍同时计算 MD5 与 SHA256（code-review-2026-11-23 #7）。
//
// 旧路径 GetFileMD5 + GetFileSHA256 各完整读一遍文件，1GB 项目一次缓存
// 重建 = 2GB IO。MultiWriter 单次 io.Copy 同时喂两个哈希器，IO 减半。
func GetFileMD5AndSHA256(filename string) (string, string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	md5Handle := md5.New()
	sha256Handle := sha256.New()
	w := io.MultiWriter(md5Handle, sha256Handle)
	if _, err := io.Copy(w, f); err != nil {
		return "", "", err
	}
	return fmt.Sprintf("%x", md5Handle.Sum(nil)), fmt.Sprintf("%x", sha256Handle.Sum(nil)), nil
}
