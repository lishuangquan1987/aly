package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

type cliClient struct {
	exe string
}

type cliOutput struct {
	IsSuccess bool            `json:"isSuccess"`
	ErrorMsg  string          `json:"errorMsg"`
	Data      json.RawMessage `json:"data"`
}

type FileStatusItem struct {
	RelativePath string `json:"relativePath"`
	Status       string `json:"status"`
	LocalMd5     string `json:"localMd5"`
	LocalSize    int64  `json:"localSize"`
	ServerMd5    string `json:"serverMd5"`
	ServerSize   int64  `json:"serverSize"`
}

type StatusData struct {
	Staged    []FileStatusItem `json:"staged"`
	Unstaged  []FileStatusItem `json:"unstaged"`
	Unchanged []FileStatusItem `json:"unchanged"`
}

type ChangeLog struct {
	ID      int      `json:"id"`
	Project int      `json:"project_id"`
	Version string   `json:"version"`
	Logs    []string `json:"logs"`
	Time    string   `json:"time"`
}

type UploadProgress struct {
	Index    int    `json:"index"`
	Total    int    `json:"total"`
	File     string `json:"file"`
	Status   string `json:"status"`
	FileSize int64  `json:"file_size"`
	Error    string `json:"error"`
}

type ProjectInfo struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	ForceUpdate bool   `json:"force_update"`
}

func (c *cliClient) available() bool { return c != nil && c.exe != "" }

func (c *cliClient) runRaw(ctx context.Context, workDir string, args []string) ([]byte, error) {
	if !c.available() {
		return nil, errCLIMissing
	}
	log.Printf("aly-publish %v", args)
	cmd := exec.CommandContext(ctx, c.exe, args...)
	hideConsole(cmd)
	if workDir != "" {
		cmd.Dir = workDir
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	cmd.WaitDelay = 5 * time.Second
	err := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return nil, context.DeadlineExceeded
		}
		return nil, context.Canceled
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		log.Printf("aly-publish %v failed: %s", args, msg)
		return nil, fmt.Errorf("%s", msg)
	}
	return stdout.Bytes(), nil
}

// runJSONOutput runs a command with --json, checks isSuccess (the CLI exits 0
// even on business errors), and returns the data payload.
func (c *cliClient) runJSONOutput(ctx context.Context, workDir string, args []string) (json.RawMessage, error) {
	full := append([]string{"--json"}, args...)
	raw, err := c.runRaw(ctx, workDir, full)
	if err != nil {
		return nil, err
	}
	var parsed cliOutput
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("解析 aly-publish 输出失败: %w", err)
	}
	if !parsed.IsSuccess {
		msg := strings.TrimSpace(parsed.ErrorMsg)
		if msg == "" {
			msg = "aly-publish 执行失败"
		}
		log.Printf("aly-publish %v returned isSuccess=false: %s", args, msg)
		return nil, fmt.Errorf("%s", msg)
	}
	return parsed.Data, nil
}

func (c *cliClient) runJSON(ctx context.Context, workDir string, args []string, out any) error {
	data, err := c.runJSONOutput(ctx, workDir, args)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if len(data) == 0 || string(data) == "null" {
		return fmt.Errorf("aly-publish 未返回数据")
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("解析 aly-publish data 失败: %w", err)
	}
	return nil
}

func (c *cliClient) checked(ctx context.Context, workDir string, args []string) error {
	_, err := c.runJSONOutput(ctx, workDir, args)
	return err
}

func (c *cliClient) status(ctx context.Context, projectPath string) (StatusData, error) {
	var data StatusData
	err := c.runJSON(ctx, projectPath, []string{"status"}, &data)
	return data, err
}

func (c *cliClient) logs(ctx context.Context, projectPath string, limit int) ([]ChangeLog, error) {
	var data []ChangeLog
	err := c.runJSON(ctx, projectPath, []string{"log", "--limit", fmt.Sprintf("%d", limit)}, &data)
	return data, err
}

func csvFlag(args []string, flag string, values []string) []string {
	if len(values) > 0 {
		return append(args, flag, strings.Join(values, ","))
	}
	return args
}

func (c *cliClient) configSet(ctx context.Context, projectPath, key, value string) error {
	return c.checked(ctx, projectPath, []string{"config", "set", key, value})
}

func (c *cliClient) initProject(
	ctx context.Context,
	projectPath, serverURL, projectName string,
	ignoreFolders, ignoreFiles, unCopyFolders, unCopyFiles []string,
) error {
	args := []string{"config", "init", "--server", serverURL, "--project", projectName}
	args = csvFlag(args, "--ignore-folders", ignoreFolders)
	args = csvFlag(args, "--ignore-files", ignoreFiles)
	args = csvFlag(args, "--un-copy-folders", unCopyFolders)
	args = csvFlag(args, "--un-copy-files", unCopyFiles)
	return c.checked(ctx, projectPath, args)
}

func (c *cliClient) projectList(ctx context.Context, serverURL string) ([]ProjectInfo, error) {
	var out []ProjectInfo
	args := []string{"project", "list"}
	if serverURL != "" {
		args = append(args, "--server", serverURL)
	}
	err := c.runJSON(ctx, "", args, &out)
	return out, err
}

func (c *cliClient) projectCreate(
	ctx context.Context,
	serverURL, name, title string,
	forceUpdate bool,
	ignoreFolders, ignoreFiles []string,
) (ProjectInfo, error) {
	var out ProjectInfo
	args := []string{"project", "create", "--server", serverURL, "--name", name, "--title", title}
	if forceUpdate {
		args = append(args, "--force-update")
	}
	args = csvFlag(args, "--ignore-folders", ignoreFolders)
	args = csvFlag(args, "--ignore-files", ignoreFiles)
	err := c.runJSON(ctx, "", args, &out)
	return out, err
}

func (c *cliClient) projectUpdate(
	ctx context.Context,
	serverURL, name, title string,
	forceUpdate bool,
	ignoreFolders, ignoreFiles []string,
) error {
	args := []string{"project", "update", "--server", serverURL, "--name", name}
	if title != "" {
		args = append(args, "--title", title)
	}
	if forceUpdate {
		args = append(args, "--force-update")
	}
	args = csvFlag(args, "--ignore-folders", ignoreFolders)
	args = csvFlag(args, "--ignore-files", ignoreFiles)
	return c.checked(ctx, "", args)
}

func (c *cliClient) projectInfo(ctx context.Context, serverURL, projectName string) (ProjectInfo, error) {
	var info ProjectInfo
	args := []string{"project", "info"}
	if serverURL != "" {
		args = append(args, "--server", serverURL)
	}
	args = append(args, "--name", projectName)
	err := c.runJSON(ctx, "", args, &info)
	return info, err
}

func (c *cliClient) addAll(ctx context.Context, projectPath string) error {
	return c.checked(ctx, projectPath, []string{"add", "--all"})
}

func (c *cliClient) resetAll(ctx context.Context, projectPath string) error {
	return c.checked(ctx, projectPath, []string{"reset", "--all"})
}

func (c *cliClient) addFiles(ctx context.Context, projectPath string, files []string) error {
	if len(files) == 0 {
		return nil
	}
	return c.checked(ctx, projectPath, append([]string{"add", "--"}, files...))
}

func (c *cliClient) resetFiles(ctx context.Context, projectPath string, files []string) error {
	if len(files) == 0 {
		return nil
	}
	return c.checked(ctx, projectPath, append([]string{"reset", "--"}, files...))
}

func (c *cliClient) configSetArray(ctx context.Context, projectPath, key string, add bool, item string) error {
	flag := "--remove"
	if add {
		flag = "--add"
	}
	return c.checked(ctx, projectPath, []string{"config", "set-array", key, flag, item})
}

// push runs `push --json` and streams each JSON line to onLine. It returns
// context.Canceled / context.DeadlineExceeded, or the CLI's error otherwise.
func (c *cliClient) push(
	ctx context.Context,
	projectPath, version, message, afterApply string,
	force *bool,
	onLine func(string),
) error {
	if !c.available() {
		return errCLIMissing
	}
	args := []string{"--json", "push", "--version", version, "--message", message}
	log.Printf("aly-publish push %s (%s)", version, projectPath)
	if force != nil {
		if *force {
			args = append(args, "--set-force-update")
		} else {
			args = append(args, "--clear-force-update")
		}
	}
	if strings.TrimSpace(afterApply) != "" {
		args = append(args, "--after-apply-update-script", afterApply)
	}
	cmd := exec.CommandContext(ctx, c.exe, args...)
	hideConsole(cmd)
	cmd.Dir = projectPath
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Cancel = func() error {
		if cmd.Process != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if onLine != nil {
			onLine(scanner.Text())
		}
	}
	scanErr := scanner.Err()
	waitErr := cmd.Wait()
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return context.DeadlineExceeded
		}
		return context.Canceled
	}
	if waitErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = waitErr.Error()
		}
		log.Printf("aly-publish push failed: %s", msg)
		return fmt.Errorf("%s", msg)
	}
	if scanErr != nil {
		return fmt.Errorf("读取发布进度失败: %w", scanErr)
	}
	return nil
}
