package model

// FileDiff 表示一个文件的差异信息（内部使用）
type FileDiff struct {
	RelativePath string
	LocalMD5     string
	LocalSize    int64
	LocalSHA256  string
	ServerMD5    string
	ServerSize   int64
	ServerSHA256 string
}
