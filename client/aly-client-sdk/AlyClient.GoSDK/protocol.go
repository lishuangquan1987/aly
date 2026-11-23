package alyclient

import "strings"

// Response 是所有命令的统一响应。业务失败不会返回 error：
// IsSuccess=false 时 ErrorMsg 说明原因；Data 为零值。
type Response[T any] struct {
	IsSuccess bool   `json:"isSuccess"`
	ErrorMsg  string `json:"errorMsg"`
	Data      T      `json:"data"`

	// reason 是 download_update 失败终态的机器码（协议 #13，不参与 JSON）：
	// already_latest / partial_failure / materialize_failed / version_state_failed。
	// 宿主机器判定以此为准，errorMsg 文案匹配仅作旧版客户端兼容回退。
	reason string
}

// 下载失败原因机器码（DownloadUpdateData.reason，协议 #13）。
const (
	DownloadFailReasonAlreadyLatest      = "already_latest"
	DownloadFailReasonPartialFailure     = "partial_failure"
	DownloadFailReasonMaterializeFailed  = "materialize_failed"
	DownloadFailReasonVersionStateFailed = "version_state_failed"
)

// Reason 返回 download_update 失败终态的机器码（非下载命令返回空串）。
func (r Response[T]) Reason() string { return r.reason }

// Empty 用于没有 data 载荷的命令（apply_update、download_update 完成）。
type Empty struct{}

// OK 构造成功响应。
func OK[T any](data T) Response[T] {
	return Response[T]{IsSuccess: true, ErrorMsg: "", Data: data}
}

// NG 构造失败响应。
func NG[T any](errMsg string) Response[T] {
	return Response[T]{IsSuccess: false, ErrorMsg: errMsg}
}

// isAlreadyLatest 判断下载失败是否"已是最新"（#13）：优先 reason 机器码，
// 旧版客户端无 reason 时回退 errorMsg 文案匹配。
func isAlreadyLatest(r Response[Empty]) bool {
	if r.reason == DownloadFailReasonAlreadyLatest {
		return true
	}
	return r.reason == "" && strings.Contains(r.ErrorMsg, "already at latest version")
}
