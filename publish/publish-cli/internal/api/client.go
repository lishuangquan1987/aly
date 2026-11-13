package api

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"aly/publish-cli/pkg/models"
)

// Client HTTP API 客户端
type Client struct {
	ServerURL string
	hc        *http.Client
	chunkSize int64
}

// NewClient 创建客户端。
//
// 上传用"每请求一条新连接"的 http client（DisableKeepAlives），与客户端下载侧同款策略：
// 中间的安全设备/代理把连接强制掐断时（实测报 `wsarecv: ... forcibly closed by the remote host`），
// 若复用连接池，5 次重试会全部撞在同一条死连接上；每次新建连接重试才真正有意义。
// 连接级超时（拨号/TLS/响应头）保证单次请求不会无限挂着，整体 Timeout 留足上传窗口。
func NewClient(serverURL string) *Client {
	c := &Client{
		// 去掉末尾 "/"，避免拼出 "http://host//api/..." 双斜杠路径
		ServerURL: strings.TrimRight(serverURL, "/"),
		hc: &http.Client{
			Timeout: 600 * time.Second,
			Transport: &http.Transport{
				Dial: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).Dial,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 60 * time.Second,
				DisableKeepAlives:     true,
			},
		},
		chunkSize: resolveChunkSize(),
	}
	c.printTransferSetup()
	return c
}

// printTransferSetup 把生效的上传参数写到 stderr（GUI 会收进日志）。
// 排查"上传被中间设备断链"时，第一件事就是确认实际用的分片大小与连接策略。
func (c *Client) printTransferSetup() {
	fmt.Fprintf(os.Stderr, "upload setup: chunk=%d bytes, keep-alives=off, dial=30s, resp-head=60s\n", c.chunkSize)
}

// ─── 通用方法 ──────────────────────────────────────────────────────────

// get 发起 GET 并解析统一信封。
//
// 审查 publish 轻微③：GET 是幂等的，旧实现单发——弱网下一次瞬断就让 status/log
// 这类只读命令整条失败。这里加与上传同款的 transient 重试。
func (c *Client) get(path string, result interface{}) error {
	var lastErr error
	for attempt := 0; attempt < maxUploadRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(uploadRetryBackoff(attempt))
		}
		err := c.getOnce(path, result)
		if err == nil {
			return nil
		}
		if strings.Contains(err.Error(), "parse response") || !isTransientUploadError(err) {
			return err
		}
		lastErr = err
	}
	return fmt.Errorf("GET %s 重试 %d 次仍失败: %w", path, maxUploadRetries, lastErr)
}

func (c *Client) getOnce(path string, result interface{}) error {
	resp, err := c.hc.Get(c.ServerURL + path)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("GET %s returned HTTP %d (failed to read body: %v)", path, resp.StatusCode, readErr)
		}
		return fmt.Errorf("GET %s returned HTTP %d: %s", path, resp.StatusCode, string(body))
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("GET %s read body: %w", path, readErr)
	}
	var cr models.CommonResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return fmt.Errorf("GET %s parse response: %w", path, err)
	}
	if !cr.IsSuccess {
		return fmt.Errorf("GET %s: %s", path, cr.ErrorMsg)
	}
	if result != nil && cr.Data != nil {
		return json.Unmarshal(cr.Data, result)
	}
	return nil
}

func (c *Client) post(path string, reqBody interface{}, result interface{}) error {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("POST %s marshal request: %w", path, err)
	}
	resp, err := c.hc.Post(c.ServerURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("POST %s returned HTTP %d (failed to read body: %v)", path, resp.StatusCode, readErr)
		}
		return fmt.Errorf("POST %s returned HTTP %d: %s", path, resp.StatusCode, string(body))
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("POST %s read body: %w", path, readErr)
	}
	var cr models.CommonResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return fmt.Errorf("POST %s parse response: %w", path, err)
	}
	if !cr.IsSuccess {
		return fmt.Errorf("POST %s: %s", path, cr.ErrorMsg)
	}
	if result != nil && cr.Data != nil {
		return json.Unmarshal(cr.Data, result)
	}
	return nil
}

// ─── Project API ───────────────────────────────────────────────────────

// GetAllProjects 获取所有项目
func (c *Client) GetAllProjects() ([]models.Project, error) {
	var projects []models.Project
	if err := c.get("/api/project/get_all_projects", &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// CreateProject 创建项目
func (c *Client) CreateProject(req models.ProjectConfigRequest) (*models.Project, error) {
	var project models.Project
	if err := c.post("/api/project/create_project", req, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// UpdateProject 更新项目配置
func (c *Client) UpdateProject(req models.ProjectConfigRequest) error {
	return c.post("/api/project/update_project", req, nil)
}

// SetProjectForceUpdate 只更新项目的强制更新标记，不涉及其他字段
func (c *Client) SetProjectForceUpdate(projectName string, forceUpdate bool) error {
	return c.post("/api/project/set_force_update", map[string]interface{}{
		"projectName": projectName,
		"forceUpdate": forceUpdate,
	}, nil)
}

// DeleteProject 软删除项目
func (c *Client) DeleteProject(projectName string) error {
	path := fmt.Sprintf("/api/project/delete_project/%s", url.PathEscape(projectName))
	return c.post(path, struct{}{}, nil)
}

// GetProjectChangeLogs 获取变更日志（按项目名称）
func (c *Client) GetProjectChangeLogs(projectName string) ([]models.ProjectChangeLog, error) {
	var logs []models.ProjectChangeLog
	path := fmt.Sprintf("/api/project/get_project_change_logs/%s", url.PathEscape(projectName))
	if err := c.get(path, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

// GetReleaseManifest 获取项目当前发布清单（审查 B7/A2）。
//
// 清单只在 PublishVersion 成功时写入，因此"文件在清单里"等价于
// "该文件已随某个版本正式发布"——这是区分"上传成功但版本没建成"的关键标记。
// 老服务器没有该接口时返回错误，调用方降级为旧行为。
func (c *Client) GetReleaseManifest(projectName string) ([]string, error) {
	var manifest struct {
		Files []string `json:"files"`
	}
	path := fmt.Sprintf("/api/file/get_release_manifest/%s", url.PathEscape(projectName))
	if err := c.get(path, &manifest); err != nil {
		return nil, err
	}
	return manifest.Files, nil
}

// DeleteFile 删除服务端项目内的一个文件（审查 B6：删除同步链路）。
// 服务端语义为幂等删除（文件不存在也返回成功）。
func (c *Client) DeleteFile(projectName, relativeFileName string) error {
	return c.post("/api/file/delete_file", map[string]string{
		"projectName":      projectName,
		"relativeFileName": relativeFileName,
	}, nil)
}

// GetProjectOSInfo 获取服务端系统信息（按项目名称）
func (c *Client) GetProjectOSInfo(projectName string) ([]models.ServerOSInfo, error) {
	var info []models.ServerOSInfo
	path := fmt.Sprintf("/api/project/get_project_os_info/%s", url.PathEscape(projectName))
	if err := c.get(path, &info); err != nil {
		return nil, err
	}
	return info, nil
}

// GetServerInfo 获取服务端系统信息（无需指定项目）
func (c *Client) GetServerInfo() ([]models.ServerOSInfo, error) {
	var info []models.ServerOSInfo
	if err := c.get("/api/server/info", &info); err != nil {
		return nil, err
	}
	return info, nil
}

// PublishVersion 发布新版本
func (c *Client) PublishVersion(req models.PublishVersionRequest) (*models.ProjectChangeLog, error) {
	var log models.ProjectChangeLog
	if err := c.post("/api/project/publish_version", req, &log); err != nil {
		return nil, err
	}
	return &log, nil
}

// ─── File API ──────────────────────────────────────────────────────────

// GetAllFiles 获取服务端文件列表（按项目名称）
func (c *Client) GetAllFiles(projectName string) ([]models.FileInfo, error) {
	var files []models.FileInfo
	path := fmt.Sprintf("/api/file/get_all_files/%s", url.PathEscape(projectName))
	if err := c.get(path, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// ─── 分片上传常量 ──────────────────────────────────────────────────────

// defaultChunkSize 默认分片大小：128 KiB。
// 分片越小，单次请求越容易穿过中间的安全设备/代理（实测环境里大请求容易被强制断链）；
// 分片越大，请求数越少。可用环境变量 ALY_CHUNK_SIZE（字节数）覆盖，生效值会写进上传日志。
const defaultChunkSize = 128 * 1024

// chunckSizeEnviron 覆盖分片大小的环境变量名（值为字节数；非法值退回默认值）。
const chunckSizeEnviron = "ALY_CHUNK_SIZE"

// resolveChunkSize 解析生效的分片大小：环境变量优先，取不到或非法就用默认值。
func resolveChunkSize() int64 {
	raw := envar(chunckSizeEnviron)
	if raw == "" {
		return defaultChunkSize
	}
	if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && n > 0 {
		return int64(n)
	}
	return defaultChunkSize
}

// envar 返回环境变量 name 的值（取不到返回空串）。
//
// 审查 publish 轻微①：旧实现手写遍历 os.Environ() 做**大小写敏感**前缀匹配——
// Windows 环境变量名本就不区分大小写，用户设 `aly_chunk_size` 会静默无效。
// os.Getenv 在 Windows 上即大小写不敏感，直接用即可。
func envar(name string) string {
	return os.Getenv(name)
}

// ─── 上传重试 ──────────────────────────────────────────────────────────

// maxUploadRetries 单文件直传 / 单分片上传的重试次数上限（原仅分片路径有重试）
const maxUploadRetries = 5

// uploadRetryBackoff 第 attempt（从 1 起）次重试前的退避间隔：
// 400ms → 800ms → 1600ms → 3200ms（累计约 6s）。
func uploadRetryBackoff(attempt int) time.Duration {
	return time.Duration(200*(1<<attempt)) * time.Millisecond
}

// isTransientUploadError 判断是否为可重试的临时网络错误。
//
// 除显式的连接字样外，必须覆盖"对端直接断链"这一类：服务端读超时
// （server/main.go ReadTimeout = 60s）或链路中断时，客户端往往只看到一个
// 光光的 "EOF"/"wsarecv"，不含 closed pipe / reset 等字样。漏掉它会让分片
// 在第一片就放弃——5 次重试形同虚设（2026-11-06 实测：远程 1MiB 分片撞上
// 60s 读超时窗口被断链，报 `upload chunk 0/2: ... EOF`，却未重试）。
// 判定统一转小写，兼容措辞大小写不一致的实现。
func isTransientUploadError(err error) bool {
	msg := strings.ToLower(err.Error())
	// 审查 M15：服务端 5xx（网关/临时不可用）对上传是可重试的；
	// 旧实现不识别，complete 这类单发请求遇到 502 就直接失败。
	for _, code := range []string{"http 500", "http 502", "http 503", "http 504"} {
		if strings.Contains(msg, code) {
			return true
		}
	}
	return strings.Contains(msg, "closed pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "forcibly closed") ||
		strings.Contains(msg, "timeout") ||
		// 对端断链 / 读超时断链的常见观测
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "wsarecv") ||
		strings.Contains(msg, "wsasend") ||
		strings.Contains(msg, "reset by peer") ||
		strings.Contains(msg, "connection abort")
}

// UploadFile 上传单个文件：小文件直接上传，大文件分片上传
func (c *Client) UploadFile(localPath, projectName, relativeFileName string) error {
	fi, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if fi.Size() <= c.chunkSize {
		return c.uploadFileDirect(localPath, projectName, relativeFileName)
	}
	return c.uploadFileChunked(localPath, projectName, relativeFileName, fi.Size())
}

// uploadFileDirect 直接上传小文件（内存缓冲，无 io.Pipe；带重试）。
//
// 原实现一次不成即失败，与分片路径的重试口径不一——小文件同样会撞上服务端
// 读超时断链 / 瞬时网络错误，一次失败就整文件报错。这里补齐同等重试。
// 文件内容只读一次并复用缓冲，重试不重复读盘。
func (c *Client) uploadFileDirect(localPath, projectName, relativeFileName string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	name := filepath.Base(localPath)

	var lastErr error
	for attempt := 0; attempt < maxUploadRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(uploadRetryBackoff(attempt))
		}
		err := c.uploadFileOnce(data, name, projectName, relativeFileName)
		if err == nil {
			return nil
		}
		if !isTransientUploadError(err) {
			return err
		}
		lastErr = err
	}
	return fmt.Errorf("upload file failed after %d retries (chunk=%d bytes): %w; hint: set ALY_CHUNK_SIZE to try a smaller chunk", maxUploadRetries, c.chunkSize, lastErr)
}

// uploadFileOnce 单次直传（内存缓冲，无 io.Pipe）
func (c *Client) uploadFileOnce(data []byte, fileName, projectName, relativeFileName string) error {
	body := multipartFormBuffer(data, fileName, map[string]string{
		"projectName":      projectName,
		"relativeFileName": relativeFileName,
	})

	resp, err := c.hc.Post(c.ServerURL+"/api/file/upload_file", body.contentType, &body.buf)
	if err != nil {
		return fmt.Errorf("upload file: %w", err)
	}
	defer resp.Body.Close()

	return checkResponse(resp)
}

// uploadFileChunked 分片上传大文件
func (c *Client) uploadFileChunked(localPath, projectName, relativeFileName string, fileSize int64) error {
	totalChunks := int((fileSize + c.chunkSize - 1) / c.chunkSize)

	// 审查 M3：完整文件 MD5 随 complete 一起上报，服务端合并后比对，
	// 从根上消灭"合并结果与发布者意图不一致却静默下发"的一类问题。
	wantMD5, err := fileMD5(localPath)
	if err != nil {
		return fmt.Errorf("compute md5: %w", err)
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// 顺序上传分片（每个分片有独立重试）
	if err := c.uploadAllChunks(file, fileSize, projectName, relativeFileName, totalChunks, nil); err != nil {
		return err
	}

	// 通知服务端合并分片。审查 M15：
	//   · complete 也要有 transient 重试（旧实现单发，断链即"最后一步前功尽弃"）；
	//   · 服务端缺片时返回 HTTP 200 + chunksComplete=false，旧实现只看 IsSuccess
	//     就把"未合并"当成功——文件静默缺失。这里解析 missing 并只重传缺失分片。
	missing, err := c.completeChunksWithRetry(projectName, relativeFileName, totalChunks, wantMD5)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		// 只重传缺失分片，然后重新 complete（最多两轮，避免死循环）
		if err := c.uploadAllChunks(file, fileSize, projectName, relativeFileName, totalChunks, missing); err != nil {
			return err
		}
		missing, err = c.completeChunksWithRetry(projectName, relativeFileName, totalChunks, wantMD5)
		if err != nil {
			return err
		}
		if len(missing) > 0 {
			return fmt.Errorf("服务端仍缺分片: %v（已重传一轮；请检查网络或调小 ALY_CHUNK_SIZE）", missing)
		}
	}
	return nil
}

// uploadAllChunks 上传全部分片；only 非空时只上传指定的分片下标
func (c *Client) uploadAllChunks(file *os.File, fileSize int64, projectName, relativeFileName string, totalChunks int, only []int) error {
	selected := func(i int) bool {
		if len(only) == 0 {
			return true
		}
		for _, x := range only {
			if x == i {
				return true
			}
		}
		return false
	}
	for i := 0; i < totalChunks; i++ {
		if !selected(i) {
			continue
		}
		offset := int64(i) * c.chunkSize
		remaining := fileSize - offset
		chunkLen := int(c.chunkSize)
		if remaining < int64(c.chunkSize) {
			chunkLen = int(remaining)
		}
		if chunkLen <= 0 {
			break
		}

		buf := make([]byte, chunkLen)
		if _, err := file.ReadAt(buf, offset); err != nil {
			return fmt.Errorf("read chunk %d: %w", i, err)
		}

		if err := c.uploadChunkWithRetry(buf, projectName, relativeFileName, i, totalChunks); err != nil {
			return fmt.Errorf("upload chunk %d/%d: %w", i, totalChunks, err)
		}
	}
	return nil
}

// fileMD5 计算文件 MD5（分块读取，避免大文件一次性读入内存）
func fileMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	buf := make([]byte, 1<<20)
	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// completeChunksWithRetry 通知服务端合并（带 transient 重试 + 缺片解析）
// 返回仍缺失的分片下标（空切片表示合并成功）。
func (c *Client) completeChunksWithRetry(projectName, relativeFileName string, totalChunks int, wantMD5 string) ([]int, error) {
	var lastErr error
	for attempt := 0; attempt < maxUploadRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(uploadRetryBackoff(attempt))
		}
		missing, err := c.completeChunksOnce(projectName, relativeFileName, totalChunks, wantMD5)
		if err == nil {
			return missing, nil
		}
		if !isTransientUploadError(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, fmt.Errorf("chunk complete failed after %d retries: %w", maxUploadRetries, lastErr)
}

// completeChunksOnce 单次 complete：解析服务端返回的 chunksComplete/missingChunks。
func (c *Client) completeChunksOnce(projectName, relativeFileName string, totalChunks int, wantMD5 string) ([]int, error) {
	fields := map[string]string{
		"projectName":      projectName,
		"relativeFileName": relativeFileName,
		"totalChunks":      fmt.Sprintf("%d", totalChunks),
	}
	if wantMD5 != "" {
		fields["md5"] = wantMD5
	}
	body := multipartFormBuffer(nil, "", fields)

	resp, err := c.hc.Post(c.ServerURL+"/api/file/upload_chunk_complete", body.contentType, &body.buf)
	if err != nil {
		return nil, fmt.Errorf("chunk complete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("chunk complete: HTTP %d: %s", resp.StatusCode, string(respBody))
	}
	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("read complete response: %w", readErr)
	}
	var cr models.ChunkCompleteResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return nil, fmt.Errorf("parse complete response: %w", err)
	}
	if !cr.IsSuccess {
		return nil, fmt.Errorf("%s", cr.ErrorMsg)
	}
	// 审查 M15 核心：IsSuccess=true 不代表合并成功
	if !cr.Data.ChunksComplete {
		return cr.Data.MissingChunks, nil
	}
	return nil, nil
}

// uploadChunkWithRetry 上传单个分片（带重试）
func (c *Client) uploadChunkWithRetry(data []byte, projectName, relativeFileName string, chunkIndex, totalChunks int) error {
	var lastErr error
	for attempt := 0; attempt < maxUploadRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(uploadRetryBackoff(attempt))
		}
		err := c.uploadChunkOnce(data, projectName, relativeFileName, chunkIndex, totalChunks)
		if err == nil {
			return nil
		}
		if !isTransientUploadError(err) {
			return err
		}
		lastErr = err
	}
	return fmt.Errorf("chunk %d/%d failed after %d retries (chunk=%d bytes): %w; hint: set ALY_CHUNK_SIZE to try a smaller chunk", chunkIndex, totalChunks, maxUploadRetries, c.chunkSize, lastErr)
}

// uploadChunkOnce 单次上传分片（内存缓冲，无 io.Pipe）
func (c *Client) uploadChunkOnce(data []byte, projectName, relativeFileName string, chunkIndex, totalChunks int) error {
	body := multipartFormBuffer(data, filepath.Base(relativeFileName), map[string]string{
		"projectName":      projectName,
		"relativeFileName": relativeFileName,
		"chunkIndex":       fmt.Sprintf("%d", chunkIndex),
		"totalChunks":      fmt.Sprintf("%d", totalChunks),
	})

	resp, err := c.hc.Post(c.ServerURL+"/api/file/upload_chunk", body.contentType, &body.buf)
	if err != nil {
		return fmt.Errorf("upload chunk: %w", err)
	}
	defer resp.Body.Close()

	return checkResponse(resp)
}

// ─── multipart 辅助 ─────────────────────────────────────────────────────

type multipartForm struct {
	contentType string
	buf         bytes.Buffer
}

// multipartFormBuffer 构造 multipart/form-data 请求体（完全在内存中，无 io.Pipe）
// fileData 为 nil 时跳过 file 字段（用于 complete 请求）
func multipartFormBuffer(fileData []byte, fileName string, extraFields map[string]string) multipartForm {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if fileData != nil {
		part, _ := writer.CreateFormFile("file", fileName)
		part.Write(fileData)
	}

	for key, value := range extraFields {
		writer.WriteField(key, value)
	}

	writer.Close()
	return multipartForm{contentType: writer.FormDataContentType(), buf: buf}
}

// checkResponse 检查通用响应
func checkResponse(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("HTTP %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("read response: %w", readErr)
	}
	var cr models.CommonResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if !cr.IsSuccess {
		return fmt.Errorf("%s", cr.ErrorMsg)
	}
	return nil
}
