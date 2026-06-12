// +build ignore

package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- DTO ----------

type CommonResponse struct {
	IsSuccess bool            `json:"isSuccess"`
	ErrorMsg  string          `json:"errorMsg"`
	Data      json.RawMessage `json:"data"`
}

type Project struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	Version       string   `json:"version"`
	ForceUpdate   bool     `json:"force_update"`
	IgnoreFolders []string `json:"ignore_folders"`
	IgnoreFiles   []string `json:"ignore_files"`
	CreatedAt     string   `json:"created_at"`
	IsDeleted     bool     `json:"is_deleted"`
}

type ChangeLog struct {
	ID        int      `json:"id"`
	Version   string   `json:"version"`
	Logs      []string `json:"logs"`
	Time      string   `json:"time"`
	CreatedAt string   `json:"created_at"`
	IsDeleted bool     `json:"is_deleted"`
}

type FileInfo struct {
	FileAbsolutePath string `json:"fileAbsolutePath"`
	FileRelativePath string `json:"fileRelativePath"`
	LastUpdateTime   string `json:"lastUpdateTime"`
	FileSize         int64  `json:"fileSize"`
	MD5              string `json:"md5"`
	SHA256           string `json:"sha256"`
}

type CreateProjectReq struct {
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	IsForceUpdate bool     `json:"isForceUpdate"`
	IgnoreFolders []string `json:"ignoreFolders"`
	IgnoreFiles   []string `json:"ignoreFiles"`
}

// ---------- State ----------

type MockServer struct {
	mu           sync.Mutex
	projects     []Project
	changeLogs   []ChangeLog
	nextPID      int
	nextCLID     int
	dataDir      string // where uploaded files are stored
}

func NewMockServer(dataDir string) *MockServer {
	os.MkdirAll(dataDir, 0755)
	return &MockServer{
		dataDir: dataDir,
		nextPID: 1,
		nextCLID: 1,
	}
}

func (s *MockServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	method := r.Method

	// CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Route matching
	switch {
	case method == "POST" && path == "/api/project/create_project":
		s.createProject(w, r)
	case method == "POST" && path == "/api/project/update_project":
		s.updateProject(w, r)
	case method == "POST" && strings.HasPrefix(path, "/api/project/delete_project/"):
		s.deleteProject(w, r)
	case method == "GET" && path == "/api/project/get_all_projects":
		s.getAllProjects(w, r)
	case method == "GET" && strings.HasPrefix(path, "/api/project/get_project_change_logs/"):
		s.getChangeLogs(w, r)
	case method == "GET" && strings.HasPrefix(path, "/api/project/get_project_os_info/"):
		s.getOSInfo(w, r)
	case method == "POST" && path == "/api/file/upload_file":
		s.uploadFile(w, r)
	case method == "GET" && strings.HasPrefix(path, "/api/file/get_all_files/"):
		s.getAllFiles(w, r)
	case method == "GET" && path == "/api/file/download_file":
		s.downloadFile(w, r)
	default:
		http.NotFound(w, r)
	}
}

// ---------- Handlers ----------

func (s *MockServer) createProject(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req CreateProjectReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: err.Error()})
		return
	}
	if req.Name == "" {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: "项目名称不能为空"})
		return
	}
	// Check duplicate
	for _, p := range s.projects {
		if p.Name == req.Name && !p.IsDeleted {
			s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: fmt.Sprintf("项目名称:%s已存在", req.Name)})
			return
		}
	}

	now := time.Now().Format("2006-01-02T15:04:05Z")
	proj := Project{
		ID:            s.nextPID,
		Name:          req.Name,
		Title:         req.Title,
		Version:       "V1.0.0",
		ForceUpdate:   req.IsForceUpdate,
		IgnoreFolders: req.IgnoreFolders,
		IgnoreFiles:   req.IgnoreFiles,
		CreatedAt:     now,
		IsDeleted:     false,
	}
	s.nextPID++
	s.projects = append(s.projects, proj)

	// Create first change log
	cl := ChangeLog{
		ID:        s.nextCLID,
		Version:   "V1.0.0",
		Logs:      []string{"第一次创建"},
		Time:      time.Now().Format("2006-01-02 15:04:05"),
		CreatedAt: now,
		IsDeleted: false,
	}
	s.nextCLID++
	s.changeLogs = append(s.changeLogs, cl)

	// Create project work dir
	workDir := filepath.Join(s.dataDir, req.Name)
	os.MkdirAll(workDir, 0755)

	projJSON, _ := json.Marshal(proj)
	s.json(w, CommonResponse{IsSuccess: true, Data: projJSON})
}

func (s *MockServer) updateProject(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req struct {
		ID            int      `json:"id"`
		Title         string   `json:"title"`
		IsForceUpdate bool     `json:"isForceUpdate"`
		IgnoreFolders []string `json:"ignoreFolders"`
		IgnoreFiles   []string `json:"ignoreFiles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: err.Error()})
		return
	}
	for i, p := range s.projects {
		if p.ID == req.ID && !p.IsDeleted {
			s.projects[i].Title = req.Title
			s.projects[i].ForceUpdate = req.IsForceUpdate
			s.projects[i].IgnoreFolders = req.IgnoreFolders
			s.projects[i].IgnoreFiles = req.IgnoreFiles
			s.json(w, CommonResponse{IsSuccess: true})
			return
		}
	}
	s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: "项目不存在"})
}

func (s *MockServer) deleteProject(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
	idStr := parts[len(parts)-1]
	id, _ := strconv.Atoi(idStr)
	for i, p := range s.projects {
		if p.ID == id {
			s.projects[i].IsDeleted = true
			s.json(w, CommonResponse{IsSuccess: true})
			return
		}
	}
	s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: "项目不存在"})
}

func (s *MockServer) getAllProjects(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var active []Project
	for _, p := range s.projects {
		if !p.IsDeleted {
			active = append(active, p)
		}
	}
	data, _ := json.Marshal(active)
	s.json(w, CommonResponse{IsSuccess: true, Data: data})
}

func (s *MockServer) getChangeLogs(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
	idStr := parts[len(parts)-1]
	projectID, _ := strconv.Atoi(idStr)

	// Find project by ID
	var project *Project
	for _, p := range s.projects {
		if p.ID == projectID && !p.IsDeleted {
			project = &p
			break
		}
	}
	if project == nil {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: "项目不存在"})
		return
	}

	var logs []ChangeLog
	for _, cl := range s.changeLogs {
		if !cl.IsDeleted {
			logs = append(logs, cl)
		}
	}
	data, _ := json.Marshal(logs)
	s.json(w, CommonResponse{IsSuccess: true, Data: data})
}

func (s *MockServer) getOSInfo(w http.ResponseWriter, r *http.Request) {
	info := []map[string]interface{}{
		{"os": "windows", "platform": "windows", "goarch": "amd64", "version": "go1.10"},
	}
	data, _ := json.Marshal(info)
	s.json(w, CommonResponse{IsSuccess: true, Data: data})
}

func (s *MockServer) uploadFile(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: err.Error()})
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: err.Error()})
		return
	}
	defer file.Close()

	projectName := r.FormValue("projectName")
	relativeFileName := r.FormValue("relativeFileName")

	if projectName == "" || relativeFileName == "" {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: "projectName and relativeFileName required"})
		return
	}

	// Verify project exists
	found := false
	for _, p := range s.projects {
		if p.Name == projectName && !p.IsDeleted {
			found = true
			break
		}
	}
	if !found {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: "项目不存在"})
		return
	}

	relPath := strings.Replace(relativeFileName, "\\", "/", -1)
	absPath := filepath.Join(s.dataDir, projectName, relPath)
	os.MkdirAll(filepath.Dir(absPath), 0755)

	dst, err := os.Create(absPath)
	if err != nil {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: err.Error()})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: err.Error()})
		return
	}
	_ = written

	s.json(w, CommonResponse{IsSuccess: true})
}

func (s *MockServer) getAllFiles(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
	idStr := parts[len(parts)-1]
	projectID, _ := strconv.Atoi(idStr)

	var project *Project
	for _, p := range s.projects {
		if p.ID == projectID && !p.IsDeleted {
			project = &p
			break
		}
	}
	if project == nil {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: "项目不存在"})
		return
	}

	workDir := filepath.Join(s.dataDir, project.Name)
	var files []FileInfo

	filepath.Walk(workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		relPath, _ := filepath.Rel(workDir, path)
		relPath = strings.Replace(relPath, "\\", "/", -1)

		// Check ignore folders
		for _, ignoreFolder := range project.IgnoreFolders {
			if strings.HasPrefix(relPath, strings.Replace(ignoreFolder, "\\", "/", -1)+"/") {
				return nil
			}
			if relPath == ignoreFolder {
				return nil
			}
		}
		// Check ignore files
		for _, ignoreFile := range project.IgnoreFiles {
			if relPath == strings.Replace(ignoreFile, "\\", "/", -1) {
				return nil
			}
		}

		md5Str, _ := fileMD5(path)
		sha256Str, _ := fileSHA256(path)

		files = append(files, FileInfo{
			FileAbsolutePath: path,
			FileRelativePath: relPath,
			LastUpdateTime:   info.ModTime().Format("2006-01-02 15:04:05"),
			FileSize:         info.Size(),
			MD5:              md5Str,
			SHA256:           sha256Str,
		})
		return nil
	})

	data, _ := json.Marshal(files)
	s.json(w, CommonResponse{IsSuccess: true, Data: data})
}

func (s *MockServer) downloadFile(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path required", http.StatusNotFound)
		return
	}

	// Prevent path traversal: ensure file is within dataDir
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	absDataDir, err := filepath.Abs(s.dataDir)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
		return
	}
	if !strings.HasPrefix(absPath, absDataDir + string(filepath.Separator)) && absPath != absDataDir {
		http.Error(w, "access denied", http.StatusForbidden)
		return
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	fileName := filepath.Base(absPath)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	http.ServeFile(w, r, absPath)
}

// ---------- Helpers ----------

func (s *MockServer) json(w http.ResponseWriter, resp CommonResponse) {
	w.Header().Set("Content-Type", "application/json")
	// If Data is nil, set to null
	if resp.Data == nil {
		resp.Data = json.RawMessage("null")
	}
	json.NewEncoder(w).Encode(resp)
}

func fileMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	io.Copy(h, f)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ---------- Add Change Log Helper (via HTTP) ----------

// We add an admin endpoint to create additional change logs (for testing)
func (s *MockServer) addChangeLog(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var req struct {
		ProjectID int      `json:"projectId"`
		Version   string   `json:"version"`
		Logs      []string `json:"logs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: err.Error()})
		return
	}

	// Verify project exists
	found := false
	for _, p := range s.projects {
		if p.ID == req.ProjectID && !p.IsDeleted {
			found = true
			break
		}
	}
	if !found {
		s.json(w, CommonResponse{IsSuccess: false, ErrorMsg: "项目不存在"})
		return
	}

	cl := ChangeLog{
		ID:        s.nextCLID,
		Version:   req.Version,
		Logs:      req.Logs,
		Time:      time.Now().Format("2006-01-02 15:04:05"),
		CreatedAt: time.Now().Format("2006-01-02T15:04:05Z"),
		IsDeleted: false,
	}
	s.nextCLID++
	s.changeLogs = append(s.changeLogs, cl)

	s.json(w, CommonResponse{IsSuccess: true})
}

func main() {
	// Determine data dir
	dataDir := "./test_data"
	os.MkdirAll(dataDir, 0755)

	server := NewMockServer(dataDir)

	mux := http.NewServeMux()
	mux.Handle("/api/", server)

	// Admin endpoint for testing
	mux.HandleFunc("/admin/add_change_log", server.addChangeLog)

	// Also register a handler for the root
	http.Handle("/", mux)

	port := 2000
	fmt.Printf("Mock server listening on :%d (data dir: %s)\n", port, dataDir)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
