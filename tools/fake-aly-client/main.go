// fake-aly-client 按 spec/fixtures/scenarios/*.json 回放 aly-client 的 stdout，
// 供各语言 SDK 在无网络、无真实更新器的环境下做协议/生命周期测试。
//
// 用法：
//
//	ALY_FAKE_SCENARIO=<绝对路径>/update_available.json fake-aly-client check_update
//	fake-aly-client -scenario <path> download_update
//
// 命令之后的参数（如 --must-close-process-name、--close-timeout）一律忽略。
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type scenario struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Commands    map[string]command `json:"commands"`
}

type command struct {
	Lines       []json.RawMessage `json:"lines"`
	LineDelayMS int               `json:"line_delay_ms"`
	ExitCode    int               `json:"exit_code"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Getenv("ALY_FAKE_SCENARIO"), os.Stdout, os.Stderr))
}

func run(args []string, scenarioPath string, stdout, stderr io.Writer) int {
	cmd, path, err := parseArgs(args, scenarioPath)
	if err != nil {
		fmt.Fprintln(stderr, "fake-aly-client: "+err.Error())
		return 2
	}
	if path == "" {
		fmt.Fprintln(stderr, "fake-aly-client: 未指定场景（-scenario 或 ALY_FAKE_SCENARIO）")
		return 2
	}
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "fake-aly-client: 读取场景 %s: %v\n", path, err)
		return 2
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var sc scenario
	if err := dec.Decode(&sc); err != nil {
		fmt.Fprintf(stderr, "fake-aly-client: 解析场景 %s: %v\n", path, err)
		return 2
	}
	c, ok := sc.Commands[cmd]
	if !ok {
		fmt.Fprintf(stderr, "fake-aly-client: 场景 %s 未定义命令 %q\n", sc.Name, cmd)
		return 2
	}

	delay := time.Duration(c.LineDelayMS) * time.Millisecond
	for i, line := range c.Lines {
		if i > 0 && delay > 0 {
			time.Sleep(delay)
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, line); err != nil {
			fmt.Fprintf(stderr, "fake-aly-client: 场景 %s 的 %s 第 %d 行不是合法 JSON: %v\n", sc.Name, cmd, i+1, err)
			return 2
		}
		if _, err := stdout.Write(append(compact.Bytes(), '\n')); err != nil {
			fmt.Fprintf(stderr, "fake-aly-client: 写 stdout: %v\n", err)
			return 2
		}
	}
	return c.ExitCode
}

// parseArgs 返回命令与场景路径。命令只取第一个非 -scenario 值、非选项的参数；
// 命令之后的所有参数（SDK 转发的 flags）忽略。
func parseArgs(args []string, envPath string) (cmd, scenarioPath string, err error) {
	scenarioPath = envPath
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-scenario" || arg == "--scenario" {
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("%s 缺少参数", arg)
			}
			scenarioPath = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		cmd = arg
		break
	}
	if cmd == "" {
		return "", "", fmt.Errorf("缺少子命令")
	}
	return cmd, scenarioPath, nil
}
