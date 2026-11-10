package alyclient

// Response 是所有命令的统一响应。业务失败不会返回 error：
// IsSuccess=false 时 ErrorMsg 说明原因；Data 为零值。
type Response[T any] struct {
	IsSuccess bool   `json:"isSuccess"`
	ErrorMsg  string `json:"errorMsg"`
	Data      T      `json:"data"`
}

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
