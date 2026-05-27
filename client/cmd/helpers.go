package cmd

// stripVPrefix 去掉版本号前缀 V/v（服务端版本可能是 "V1.0.0"，本地版本可能是 "1.0.0"）
func stripVPrefix(version string) string {
	if len(version) > 0 && (version[0] == 'V' || version[0] == 'v') {
		return version[1:]
	}
	return version
}

// normalizePath 将路径中的反斜杠统一为正斜杠
func normalizePath(path string) string {
	result := make([]byte, 0, len(path))
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' {
			result = append(result, '/')
		} else {
			result = append(result, path[i])
		}
	}
	return string(result)
}
