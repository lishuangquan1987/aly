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
