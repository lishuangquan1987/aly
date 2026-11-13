package client

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aly/client/aly-client/model"
)

var httpClient = &http.Client{
	Timeout: 300 * time.Second,
	// DisableKeepAlives 避开 WinXP 下连接池复用失效；MaxIdleConns / IdleConnTimeout
	// 在此处无效（需 keep-alive 时才生效），保留作为后续启用 keep-alive 时的参考
	Transport: &http.Transport{
		DisableKeepAlives:  true,
		MaxIdleConns:       10,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: false,
	},
}

// downloadHTTPClient 下载专用 client（#5）：
// 不设整体 Timeout——Go 的 http.Client.Timeout 覆盖含读取 body 的整个请求周期，
// 单文件传输超过 300 秒（如 500MB @ 1.5MB/s）会被中途掐断并删除 .part 使续传失效，
// 慢网络下大文件永远下载不下来。改为连接级超时（拨号/TLS/响应头），
// body 传输不设硬上限，完整性由大小校验 + MD5/SHA256 双哈希兜底。
var downloadHTTPClient = &http.Client{
	// net.Dialer{Timeout} 是 Go 1.0 就有的 API（DialContext 的闭包写法兼容 Go 1.10）。
	// XP 兼容性：DisableKeepAlives 与主 client 保持一致。
	Transport: &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second, // 拨号
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 60 * time.Second, // 含服务端生成首字节的时间
		DisableKeepAlives:     true,             // 与主 client 同款 XP 兼容策略
	},
}

// doGet 执行 GET 请求并返回响应体
func doGet(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("服务器返回状态码: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	return body, nil
}

// parseResponse 解析 CommonResponse 并提取 Data 字段
func parseResponse(data []byte) (*model.CommonResponse, error) {
	var resp model.CommonResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if !resp.IsSuccess {
		return nil, fmt.Errorf("服务器错误: %s", resp.ErrorMsg)
	}

	return &resp, nil
}

// buildAPIURL 校验并构建 API URL，确保 serverURL 格式合法
func buildAPIURL(serverURL, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("API 路径不能为空")
	}
	// 自动补全协议前缀，兼容旧配置（如 localhost:8080）
	if !strings.Contains(serverURL, "://") {
		serverURL = "http://" + serverURL
	}
	base, err := url.Parse(strings.TrimRight(serverURL, "/"))
	if err != nil {
		return "", fmt.Errorf("无效的服务器地址: %v", err)
	}
	if base.Host == "" {
		return "", fmt.Errorf("服务器地址缺少主机名")
	}
	// 仅使用 origin 部分，避免 serverURL 中的路径前缀导致重复
	return base.Scheme + "://" + base.Host + "/" + strings.TrimPrefix(path, "/"), nil
}

// GetProjectByName 根据项目名称获取项目（走独立接口，不再拉全量列表）
func GetProjectByName(serverURL string, name string) (*model.Project, error) {
	u, err := buildAPIURL(serverURL, "api/project/get_project_by_name/"+urlEncodePath(name))
	if err != nil {
		return nil, err
	}
	data, err := doGet(u)
	if err != nil {
		return nil, err
	}

	resp, err := parseResponse(data)
	if err != nil {
		return nil, err
	}

	var project model.Project
	if err := json.Unmarshal(resp.Data, &project); err != nil {
		return nil, fmt.Errorf("解析项目信息失败: %v", err)
	}

	return &project, nil
}

// FindProjectByName 根据项目名称查找项目（使用独立接口，不再拉全量列表）
func FindProjectByName(serverURL string, name string) (*model.Project, error) {
	project, err := GetProjectByName(serverURL, name)
	if err != nil {
		return nil, err
	}
	if project.IsDeleted {
		return nil, fmt.Errorf("未找到项目: %s", name)
	}
	return project, nil
}

// GetProjectChangeLogs 获取项目的变更日志（按项目名称）
func GetProjectChangeLogs(serverURL string, projectName string) ([]model.ProjectChangeLog, error) {
	pn := urlEncodePath(projectName)
	u, err := buildAPIURL(serverURL, "api/project/get_project_change_logs/"+pn)
	if err != nil {
		return nil, err
	}
	data, err := doGet(u)
	if err != nil {
		return nil, err
	}

	resp, err := parseResponse(data)
	if err != nil {
		return nil, err
	}

	var logs []model.ProjectChangeLog
	if err := json.Unmarshal(resp.Data, &logs); err != nil {
		return nil, fmt.Errorf("解析变更日志失败: %v", err)
	}

	return logs, nil
}

// GetAllFiles 获取项目的所有文件列表（按项目名称）
func GetAllFiles(serverURL string, projectName string) ([]model.FileInfo, error) {
	pn := urlEncodePath(projectName)
	u, err := buildAPIURL(serverURL, "api/file/get_all_files/"+pn)
	if err != nil {
		return nil, err
	}
	data, err := doGet(u)
	if err != nil {
		return nil, err
	}

	resp, err := parseResponse(data)
	if err != nil {
		return nil, err
	}

	var files []model.FileInfo
	if err := json.Unmarshal(resp.Data, &files); err != nil {
		return nil, fmt.Errorf("解析文件列表失败: %v", err)
	}

	return files, nil
}

// ReleaseManifest 服务端发布清单（PublishVersion 时固化）。
// 客户端用它执行"删除同步"：清单外（发布端已删除）的文件在物化后被清理。
type ReleaseManifest struct {
	Files []string `json:"files"`
}

// GetReleaseManifest 获取项目当前发布清单（#4 删除同步）。
// 老服务器（无该接口）返回 404 时按"无清单"处理，调用方跳过删除——不报错。
func GetReleaseManifest(serverURL string, projectName string) (*ReleaseManifest, error) {
	pn := urlEncodePath(projectName)
	u, err := buildAPIURL(serverURL, "api/file/get_release_manifest/"+pn)
	if err != nil {
		return nil, err
	}
	data, err := doGet(u)
	if err != nil {
		return nil, err
	}

	resp, err := parseResponse(data)
	if err != nil {
		return nil, err
	}

	var manifest ReleaseManifest
	if err := json.Unmarshal(resp.Data, &manifest); err != nil {
		return nil, fmt.Errorf("解析发布清单失败: %v", err)
	}
	return &manifest, nil
}

// DownloadFile 从服务端下载文件并保存到本地。
// onBytes 非空时在写入过程中周期回调累计字节数（用于单文件大下载期间
// 的进度行输出，防止 UI/SDK 因长时间无输出而误判卡死；见 download_update）。
func DownloadFile(serverURL string, serverFilePath string, localPath string, onBytes func(written int64)) error {
	u, err := buildAPIURL(serverURL, "api/file/download_file?path="+urlEncodePath(serverFilePath))
	if err != nil {
		return err
	}

	resp, err := downloadHTTPClient.Get(u) // 下载走连接级超时（#5），body 不设整体上限
	if err != nil {
		return fmt.Errorf("下载请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 原子写：先写 .part 再 rename，失败时删除临时文件。
	// 旧实现直接 os.Create(localPath)，中断会留下半截文件并被后续流程激活。
	partPath := localPath + ".part"
	f, err := os.Create(partPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}

	_, err = io.Copy(f, newProgressReader(resp.Body, onBytes))
	// M10：rename 前先落盘（Windows 即 FlushFileBuffers）——否则断电可能出现
	// "名字已原子替换、内容还没落盘"的截断文件，而 apply 阶段没有哈希复检。
	syncErr := f.Sync()
	closeErr := f.Close()
	if err != nil {
		os.Remove(partPath)
		return fmt.Errorf("写入文件失败: %v", err)
	}
	if syncErr != nil {
		os.Remove(partPath)
		return fmt.Errorf("同步文件失败: %v", syncErr)
	}
	// Close 错误（如磁盘满 flush 失败）也会丢数据，不能吞掉
	if closeErr != nil {
		os.Remove(partPath)
		return fmt.Errorf("关闭文件失败: %v", closeErr)
	}
	if err := os.Rename(partPath, localPath); err != nil {
		os.Remove(partPath)
		return fmt.Errorf("原子替换文件失败: %v", err)
	}

	return nil
}

// DownloadFileWithResume downloads a file, resuming from partial download if a .part file exists.
// largeFileThreshold is the size in bytes above which resume is attempted (e.g., 100*1024*1024 for 100MB).
// onBytes 非空时回调累计已写字节数（含续传的既有部分），语义同 DownloadFile。
func DownloadFileWithResume(serverURL string, serverFilePath string, localPath string, serverFileSize int64, largeFileThreshold int64, onBytes func(written int64)) error {
	requestURL, err := buildAPIURL(serverURL, "api/file/download_file?path="+urlEncodePath(serverFilePath))
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory failed: %v", err)
	}

	// For files larger than threshold, check for existing partial download
	partPath := localPath + ".part"
	// 轻微项：.part 已完整（大小 == 服务端大小）时直接 rename，不再整文件重下。
	// 旧条件只对"大小严格小于"的文件续传，完整 .part 反而被删掉重下（>100MB 白重传一次）。
	// 完整性由调用方随后的 MD5+SHA256 校验兜底，误判只会导致校验失败后重下，安全。
	if serverFileSize > 0 {
		if partInfo, partErr := os.Stat(partPath); partErr == nil && !partInfo.IsDir() && partInfo.Size() == serverFileSize {
			if rErr := os.Rename(partPath, localPath); rErr == nil {
				return nil
			}
			os.Remove(partPath)
		}
	}
	if serverFileSize > largeFileThreshold {
		// Check if completed file already exists
		if localInfo, localErr := os.Stat(localPath); localErr == nil && localInfo.Size() == serverFileSize {
			return nil
		}

		// Check if partial .part file exists for resume
		if partInfo, partErr := os.Stat(partPath); partErr == nil && partInfo.Size() > 0 && partInfo.Size() < serverFileSize {
			// Resume from .part file
			f, err := os.OpenFile(partPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("open part file failed: %v", err)
			}

			req, err := http.NewRequest("GET", requestURL, nil)
			if err != nil {
				f.Close()
				return fmt.Errorf("create request failed: %v", err)
			}
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", partInfo.Size()))

			resp, err := downloadHTTPClient.Do(req) // 下载走连接级超时（#5）
			if err != nil {
				f.Close()
				return fmt.Errorf("resume request failed: %v", err)
			}

			if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				f.Close()
				// Server doesn't support resume, remove .part and start over
				os.Remove(partPath)
				return DownloadFile(serverURL, serverFilePath, localPath, onBytes)
			}
			if resp.StatusCode == http.StatusOK {
				// 200（非 206）：中间代理剥离了 Range 头，返回的是完整文件。
				// 旧实现把完整文件追加到半截 .part 后（大小超预期兜底重下，浪费带宽）。
				// 正确语义（#10）：删掉 .part，走整文件重下。
				resp.Body.Close()
				f.Close()
				os.Remove(partPath)
				return DownloadFile(serverURL, serverFilePath, localPath, onBytes)
			}

			// 续传回调从既有 .part 大小起算（语义=该文件累计已写字节数）
			_, copyErr := io.Copy(f, newProgressReader(resp.Body, offsetCallback(onBytes, partInfo.Size())))
			resp.Body.Close()
			// M10：rename 前落盘（断电下防"名换内容截断"）。
			syncErr := f.Sync()
			closeErr := f.Close()

			if copyErr != nil {
				return fmt.Errorf("resume write failed: %v", copyErr)
			}
			if syncErr != nil {
				return fmt.Errorf("resume sync failed: %v", syncErr)
			}
			if closeErr != nil {
				return fmt.Errorf("resume close failed: %v", closeErr)
			}

			// Verify download is complete before renaming
			if checkInfo, checkErr := os.Stat(partPath); checkErr == nil && checkInfo.Size() == serverFileSize {
				if err := os.Rename(partPath, localPath); err != nil {
					return fmt.Errorf("rename part file failed: %v", err)
				}
				return nil
			}
			// Download not complete after resume, fall through to fresh download
			os.Remove(partPath)
		}

		// No valid .part file or resume didn't complete - remove any leftover .part
		os.Remove(partPath)
	}

	// Download to .part file, then rename atomically
	f, err := os.Create(partPath)
	if err != nil {
		return fmt.Errorf("create file failed: %v", err)
	}

	resp, err := downloadHTTPClient.Get(requestURL) // 下载走连接级超时（#5）
	if err != nil {
		f.Close()
		os.Remove(partPath)
		return fmt.Errorf("download failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		f.Close()
		os.Remove(partPath)
		return fmt.Errorf("download failed, status: %d", resp.StatusCode)
	}

	_, copyErr := io.Copy(f, newProgressReader(resp.Body, onBytes))
	resp.Body.Close()
	// M10：rename 前落盘（断电下防"名换内容截断"）。
	syncErr := f.Sync()
	closeErr := f.Close()

	if copyErr != nil {
		os.Remove(partPath)
		return fmt.Errorf("write file failed: %v", copyErr)
	}
	if syncErr != nil {
		os.Remove(partPath)
		return fmt.Errorf("sync file failed: %v", syncErr)
	}
	if closeErr != nil {
		os.Remove(partPath)
		return fmt.Errorf("close file failed: %v", closeErr)
	}

	// 校验下载完整性：与预期大小不一致说明传输不完整，删除并报错
	if serverFileSize > 0 {
		finfo, statErr := os.Stat(partPath)
		if statErr != nil || finfo.Size() != serverFileSize {
			os.Remove(partPath)
			if statErr != nil {
				return fmt.Errorf("verify part file failed: %v", statErr)
			}
			return fmt.Errorf("download incomplete: got %d bytes, expected %d", finfo.Size(), serverFileSize)
		}
	}

	if err := os.Rename(partPath, localPath); err != nil {
		return fmt.Errorf("rename part file failed: %v", err)
	}
	return nil
}

func urlEncodePath(s string) string {
	// 将反斜杠替换为正斜杠
	s = strings.Replace(s, "\\", "/", -1)
	// 对路径的每个段进行 URL 编码，保留 / 分隔符
	parts := strings.Split(s, "/")
	for i := range parts {
		// 拒绝路径遍历
		if parts[i] == ".." {
			return ""
		}
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}

// newProgressReader 包装 r，把累计读取字节数透传给 onBytes（可为 nil，此时原样返回 r）。
// 回调频率由底层 io.Copy 的缓冲粒度决定（4KB-32KB 一次），调用方自行做节流。
func newProgressReader(r io.Reader, onBytes func(written int64)) io.Reader {
	if onBytes == nil {
		return r
	}
	return &progressReader{r: r, onBytes: onBytes}
}

type progressReader struct {
	r       io.Reader
	written int64
	onBytes func(written int64)
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	if n > 0 {
		p.written += int64(n)
		p.onBytes(p.written)
	}
	return n, err
}

// offsetCallback 把"本次写入字节数"回调转换为"文件累计字节数"回调
// （续传场景：.part 已有 base 字节，本次 Range 从 0 重新计数）。
func offsetCallback(onBytes func(written int64), base int64) func(int64) {
	if onBytes == nil {
		return nil
	}
	return func(n int64) { onBytes(base + n) }
}
