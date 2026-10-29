// +build windows

package i18n

import "strings"

// 错误人话化映射（精简计划条目 2）：把 aly-client / 网络层的常见技术错误
// 翻译成用户能看懂并知道下一步做什么的文案。
//
// 匹配规则：对原始错误串做大小写不敏感的子串匹配，命中即返回翻译文案；
// 未命中返回 false（调用方走 KeyErrorGeneric，原文放"详细信息"里不丢线索）。
//
// 只覆盖确定性高的 10 条起步集合；宁缺毋滥——错误映射错了比不翻译更糟
//（例如把超时说成"文件被占用"会误导用户去关程序）。
var errorMappings = []errorMapping{
	{contains: "sharing violation", zh: "文件被其他程序占用，已尝试自动关闭占用程序，可点击重试", en: "A file is locked by another program. It has been closed automatically. You can retry now"},
	{contains: "access is denied", zh: "没有足够的权限访问文件，请以管理员身份运行或检查目录权限", en: "Access denied. Run as administrator or check folder permissions"},
	{contains: "connection refused", zh: "无法连接更新服务器，请检查服务器是否在线", en: "Cannot reach the update server. Check that it is running"},
	{contains: "connection reset", zh: "网络连接被中断，可点击重试继续（已下载部分会自动保留）", en: "The connection was interrupted. Retry to resume (finished files are kept)"},
	{contains: "timeout", zh: "网络超时，可点击重试继续（已下载部分会自动保留）", en: "Network timeout. Retry to resume (finished files are kept)"},
	{contains: "timed out", zh: "网络超时，可点击重试继续（已下载部分会自动保留）", en: "Network timeout. Retry to resume (finished files are kept)"},
	{contains: "no such host", zh: "解析服务器地址失败，请检查网络或服务器地址配置", en: "Cannot resolve the server address. Check the network or server URL"},
	{contains: "checksum mismatch", zh: "下载的文件校验不一致，已自动删除坏文件，可点击重试", en: "A downloaded file failed verification and was deleted. You can retry"},
	{contains: "disk space", zh: "磁盘空间不足，请清理后重试", en: "Not enough disk space. Free up space and retry"},
	{contains: "context deadline exceeded", zh: "操作超时，可点击重试", en: "The operation timed out. You can retry"},
}

type errorMapping struct {
	contains string
	zh       string
	en       string
}

// HumanizeError 把技术错误串映射为用户友好文案。
// 第二返回值为 false 表示未收录（调用方应用通用文案，原文放详细信息）。
func HumanizeError(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	lower := strings.ToLower(raw)
	for _, m := range errorMappings {
		if strings.Contains(lower, m.contains) {
			if current == EN {
				return m.en, true
			}
			return m.zh, true
		}
	}
	return "", false
}
