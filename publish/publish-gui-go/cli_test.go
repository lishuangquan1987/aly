package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The CLI under test prepends --json. Registering it lets this test binary
// parse the same arguments before TestMain turns the child into a fake CLI.
var _ = flag.Bool("json", false, "accept the aly-publish JSON flag in helper processes")

// TestMain reuses the current Go test binary as a real helper process. Production
// code still executes an actual child process; no command runner is mocked.
func TestMain(m *testing.M) {
	if os.Getenv("ALY_GUI_TEST_HELPER") == "1" {
		runCLIHelperProcess()
		return
	}
	os.Exit(m.Run())
}

type helperCall struct {
	RawArgs []string `json:"rawArgs"`
	Args    []string `json:"args"`
}

func runCLIHelperProcess() {
	rawArgs := os.Args[1:]
	args := rawArgs
	if len(args) > 0 && args[0] == "--json" {
		args = args[1:]
	}
	logPath := os.Getenv("ALY_GUI_TEST_ARGV_LOG")
	call := 1
	if raw, err := os.ReadFile(logPath); err == nil && len(raw) > 0 {
		call = strings.Count(string(raw), "\n") + 1
	}
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err == nil {
			_ = json.NewEncoder(f).Encode(helperCall{RawArgs: rawArgs, Args: args})
			_ = f.Close()
		}
	}
	if ready := os.Getenv("ALY_GUI_TEST_READY_FILE"); ready != "" {
		_ = os.WriteFile(ready, []byte("ready"), 0o644)
	}
	if n, _ := strconv.Atoi(os.Getenv("ALY_GUI_TEST_FAIL_ON_CALL")); n > 0 && call == n {
		fmt.Println(`{"isSuccess":false,"errorMsg":"injected call failure","data":null}`)
		os.Exit(0)
	}

	mode := os.Getenv("ALY_GUI_TEST_MODE")
	if len(args) > 0 && args[0] != "push" && strings.HasPrefix(mode, "push-") {
		// reset/status/log calls around a push still use normal JSON envelopes.
		mode = "success"
	}
	switch mode {
	case "business-failure":
		fmt.Println(`{"isSuccess":false,"errorMsg":"business rejected","data":null}`)
	case "empty-error":
		fmt.Println(`{"isSuccess":false,"errorMsg":"   ","data":null}`)
	case "bad-json":
		fmt.Println(`{not-json`)
	case "null":
		fmt.Println(`{"isSuccess":true,"errorMsg":"","data":null}`)
	case "process-failure":
		fmt.Fprintln(os.Stderr, "helper process exploded")
		os.Exit(9)
	case "block":
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt)
		<-ch
	case "push-success":
		fmt.Println(`{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":1,"file":"app.exe","status":"DONE","file_size":1}}`)
		fmt.Println(`{"isSuccess":true,"errorMsg":"","data":{"version":"V1.0.1","files":"1"}}`)
	case "push-failure":
		fmt.Println(`{"isSuccess":false,"errorMsg":"push rejected","data":null}`)
	case "push-no-result":
		// A zero exit without a final result is a distinct GUI error path.
	default:
		data := "null"
		if len(args) > 0 {
			switch args[0] {
			case "status":
				if os.Getenv("ALY_GUI_TEST_MODE") == "deleted-status" {
					data = `{"staged":[{"relativePath":"old.dll","status":"deleted"},{"relativePath":"app.exe","status":"modified"}],"unstaged":[],"unchanged":[]}`
				} else {
					data = `{"staged":[],"unstaged":[],"unchanged":[]}`
				}
			case "log":
				data = `[]`
			case "project":
				if len(args) > 1 && args[1] == "list" {
					if raw := os.Getenv("ALY_GUI_TEST_PROJECT_LIST_JSON"); raw != "" {
						data = raw
					} else {
						name := os.Getenv("ALY_GUI_TEST_PROJECT_NAME")
						if name == "" {
							name = "demo"
						}
						raw, _ := json.Marshal([]ProjectInfo{{ID: 1, Name: name, Title: "Server title", Version: "V1.0.0", ForceUpdate: true}})
						data = string(raw)
					}
				} else if len(args) > 1 && args[1] == "create" {
					name := os.Getenv("ALY_GUI_TEST_PROJECT_NAME")
					if name == "" {
						name = "demo"
					}
					raw, _ := json.Marshal(ProjectInfo{ID: 7, Name: name, Version: "V1.0.0", ForceUpdate: true})
					data = string(raw)
				} else if len(args) > 1 && args[1] == "info" {
					data = `{"id":1,"name":"demo","title":"Demo","version":"V1.0.0","force_update":true}`
				}
			}
		}
		fmt.Printf(`{"isSuccess":true,"errorMsg":"","data":%s}`+"\n", data)
	}
	os.Exit(0)
}

func newHelperCLI(t *testing.T, mode string) (*cliClient, string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "argv.jsonl")
	t.Setenv("ALY_GUI_TEST_HELPER", "1")
	t.Setenv("ALY_GUI_TEST_MODE", mode)
	t.Setenv("ALY_GUI_TEST_ARGV_LOG", logPath)
	t.Setenv("ALY_GUI_TEST_FAIL_ON_CALL", "")
	t.Setenv("ALY_GUI_TEST_READY_FILE", "")
	t.Setenv("ALY_GUI_TEST_PROJECT_NAME", "")
	t.Setenv("ALY_GUI_TEST_PROJECT_LIST_JSON", "")
	return &cliClient{exe: os.Args[0]}, logPath
}

func readHelperCalls(t *testing.T, path string) []helperCall {
	t.Helper()
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var calls []helperCall
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var call helperCall
		if err := json.Unmarshal(scanner.Bytes(), &call); err != nil {
			t.Fatalf("decode helper call: %v", err)
		}
		calls = append(calls, call)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return calls
}

func waitForTestCondition(t *testing.T, description string, condition func() bool) {
	t.Helper()
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		if condition() {
			return
		}
		select {
		case <-ticker.C:
		case <-timer.C:
			t.Fatalf("timed out waiting for %s", description)
		}
	}
}

func TestRunJSONOutputContract(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cli, logPath := newHelperCLI(t, "success")
		var got StatusData
		if err := cli.runJSON(context.Background(), "", []string{"status"}, &got); err != nil {
			t.Fatal(err)
		}
		calls := readHelperCalls(t, logPath)
		if len(calls) != 1 || len(calls[0].RawArgs) != 2 || calls[0].RawArgs[0] != "--json" || calls[0].RawArgs[1] != "status" {
			t.Fatalf("raw helper args = %+v", calls)
		}
		if len(calls[0].Args) != 1 || calls[0].Args[0] != "status" {
			t.Fatalf("helper args = %+v", calls)
		}
	})
	t.Run("business failure", func(t *testing.T) {
		cli, _ := newHelperCLI(t, "business-failure")
		_, err := cli.runJSONOutput(context.Background(), "", []string{"status"})
		if err == nil || err.Error() != "business rejected" {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("empty business error gets fallback", func(t *testing.T) {
		cli, _ := newHelperCLI(t, "empty-error")
		_, err := cli.runJSONOutput(context.Background(), "", []string{"status"})
		if err == nil || err.Error() != "aly-publish 执行失败" {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("malformed JSON", func(t *testing.T) {
		cli, _ := newHelperCLI(t, "bad-json")
		_, err := cli.runJSONOutput(context.Background(), "", []string{"status"})
		if err == nil || !strings.Contains(err.Error(), "解析 aly-publish 输出失败") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("process failure returns stderr", func(t *testing.T) {
		cli, _ := newHelperCLI(t, "process-failure")
		_, err := cli.runJSONOutput(context.Background(), "", []string{"status"})
		if err == nil || err.Error() != "helper process exploded" {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("missing CLI", func(t *testing.T) {
		cli := &cliClient{}
		_, err := cli.runJSONOutput(context.Background(), "", []string{"status"})
		if !errors.Is(err, errCLIMissing) {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestRunJSONRejectsNullData(t *testing.T) {
	cli, _ := newHelperCLI(t, "null")
	var got StatusData
	err := cli.runJSON(context.Background(), "", []string{"status"}, &got)
	if err == nil || err.Error() != "aly-publish 未返回数据" {
		t.Fatalf("error = %v", err)
	}
}

func TestRunRawCancellation(t *testing.T) {
	cli, _ := newHelperCLI(t, "block")
	ready := filepath.Join(t.TempDir(), "ready")
	t.Setenv("ALY_GUI_TEST_READY_FILE", ready)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := cli.runRaw(ctx, "", []string{"status"})
		done <- err
	}()
	waitForTestCondition(t, "helper process readiness", func() bool {
		_, err := os.Stat(ready)
		return err == nil
	})
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("runRaw did not return after cancellation")
	}
}

func TestFilesInBatches(t *testing.T) {
	for _, count := range []int{0, 1, 200, 201, 400} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			cli, logPath := newHelperCLI(t, "success")
			files := make([]string, count)
			for i := range files {
				files[i] = fmt.Sprintf("file-%03d.txt", i)
			}
			if err := cli.filesInBatches(context.Background(), "", "add", files); err != nil {
				t.Fatal(err)
			}
			calls := readHelperCalls(t, logPath)
			wantCalls := (count + maxArgsPerCommand - 1) / maxArgsPerCommand
			if len(calls) != wantCalls {
				t.Fatalf("calls = %d, want %d", len(calls), wantCalls)
			}
			var got []string
			for _, call := range calls {
				if len(call.Args) < 2 || call.Args[0] != "add" || call.Args[1] != "--" {
					t.Fatalf("bad batch prefix: %v", call.Args)
				}
				if len(call.Args)-2 > maxArgsPerCommand {
					t.Fatalf("oversized batch: %d", len(call.Args)-2)
				}
				got = append(got, call.Args[2:]...)
			}
			if strings.Join(got, ",") != strings.Join(files, ",") {
				t.Fatalf("files changed across batches: got %d want %d", len(got), len(files))
			}
		})
	}
}

func TestFilesInBatchesStopsAfterFailedBatch(t *testing.T) {
	cli, logPath := newHelperCLI(t, "success")
	t.Setenv("ALY_GUI_TEST_FAIL_ON_CALL", "2")
	files := make([]string, 401)
	for i := range files {
		files[i] = fmt.Sprintf("file-%03d.txt", i)
	}
	err := cli.filesInBatches(context.Background(), "", "add", files)
	if err == nil || !strings.Contains(err.Error(), "批次 201-400/401") {
		t.Fatalf("error = %v", err)
	}
	if calls := readHelperCalls(t, logPath); len(calls) != 2 {
		t.Fatalf("calls = %d, want stop after second batch", len(calls))
	}
}
