package model

// DiffType 表示文件差异类型
type DiffType int

const (
	DiffTypeLocalOnly  DiffType = iota // 仅本地存在
	DiffTypeServerOnly                 // 仅服务端存在
	DiffTypeModified                   // 两端都存在但内容不同
)

// FileDiff 表示一个文件的差异信息
type FileDiff struct {
	RelativePath string
	DiffType     DiffType
	LocalSize    int64
	ServerSize   int64
	LocalMD5     string
	ServerMD5    string
}
