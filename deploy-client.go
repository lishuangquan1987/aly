// deploy-client: 通过 WSL 编译 aly-client.exe (32位/XP兼容) 并部署到目标目录
//
// 用法: go run deploy-client.go
// 前置: Windows 已安装 WSL，WSL 内已安装 Go 1.10

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	wslGo     = "/usr/local/go/bin/go"
	wslGoroot = "/usr/local/go"
	wslGopath = "/home/test/go"
)

var (
	scriptDir string
	wslSrc    string // WSL 内的源码路径
	linuxSrc  string // 源码在 WSL 中的挂载路径
)

type deployTarget struct {
	win string
	wsl string
}

type projectConfig struct {
	ProjectPath string `json:"ProjectPath"`
}

func mustInit() {
	var err error
	scriptDir, err = os.Getwd()
	if err != nil {
		scriptDir, _ = os.Getwd()
	}
	if scriptDir == "" {
		scriptDir = "."
	}
	scriptDir, _ = filepath.Abs(scriptDir)

	wslSrc = wslGopath + "/src/aly/client/aly-client"
	linuxSrc = "/mnt/e/Project2026/aly/client/aly-client"
}

func wsl(cmd string) error {
	c := exec.Command("wsl", "bash", "-c", cmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func wslOutput(cmd string) (string, error) {
	c := exec.Command("wsl", "bash", "-c", cmd)
	c.Stderr = os.Stderr
	out, err := c.Output()
	return strings.TrimSpace(string(out)), err
}

func winToWsl(winPath string) string {
	if len(winPath) < 2 {
		return winPath
	}
	drive := strings.ToLower(string(winPath[0]))
	rest := strings.ReplaceAll(winPath[2:], "\\", "/")
	return "/mnt/" + drive + rest
}

func configPath() string {
	appdata := os.Getenv("LOCALAPPDATA")
	if appdata == "" {
		home, _ := os.UserHomeDir()
		appdata = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(appdata, "AlyPublish", "config.json")
}

func loadDeployTargets() []deployTarget {
	cp := configPath()
	data, err := os.ReadFile(cp)
	if err != nil {
		fmt.Printf("  [WARN] config.json not found: %s\n", cp)
		return nil
	}
	var projects []projectConfig
	if err := json.Unmarshal(data, &projects); err != nil {
		fmt.Printf("  [WARN] config.json parse error: %v\n", err)
		return nil
	}

	targets := []deployTarget{}
	seen := make(map[string]bool)

	for _, proj := range projects {
		pp := proj.ProjectPath
		if pp == "" || !filepath.IsAbs(pp) {
			continue
		}
		pp = filepath.Clean(pp)

		// Target 1: ProjectPath itself
		if !seen[pp] {
			seen[pp] = true
			targets = append(targets, deployTarget{pp, winToWsl(pp)})
		}

		// Target 2: ProjectPath/../UpdateFolder
		updateFolder := filepath.Clean(filepath.Join(pp, "..", "UpdateFolder"))
		if !seen[updateFolder] {
			seen[updateFolder] = true
			targets = append(targets, deployTarget{updateFolder, winToWsl(updateFolder)})
		}
	}
	return targets
}

func main() {
	mustInit()
	fmt.Println(strings.Repeat("=", 50))

	// ── Step 1: Copy to WSL ──
	fmt.Println("  [1/4] Copy client to WSL ...")
	if err := wsl("rm -rf " + wslSrc); err != nil {
		fmt.Printf("  [WARN] rm failed: %v\n", err)
	}
	if err := wsl("mkdir -p " + wslGopath + "/src/aly/client"); err != nil {
		fmt.Printf("  [ERROR] mkdir failed: %v\n", err)
		os.Exit(1)
	}
	if err := wsl("cp -r " + linuxSrc + " " + wslSrc); err != nil {
		fmt.Printf("  [ERROR] copy to WSL failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Done\n")

	// ── Step 2: Build (XP 32-bit) ──
	fmt.Println("  [2/4] Building aly-client.exe (GOARCH=386) ...")
	buildCmd := fmt.Sprintf(
		"cd %s && GOROOT=%s GOPATH=%s GOOS=windows GOARCH=386 %s build -ldflags=\"-s -w\" -o aly-client.exe .",
		wslSrc, wslGoroot, wslGopath, wslGo,
	)
	if err := wsl(buildCmd); err != nil {
		fmt.Printf("  [ERROR] Build failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  Build OK\n")

	// ── Step 3: Deploy to all targets ──
	targets := loadDeployTargets()
	if len(targets) == 0 {
		fmt.Println("  [3/4] No deploy targets found in config.json\n")
	} else {
		fmt.Printf("  [3/4] Deploying to %d target(s) ...\n", len(targets))
		fail := false
		for i, t := range targets {
			fmt.Printf("         [%d/%d] %s\n", i+1, len(targets), t.win)
			if err := wsl("mkdir -p " + t.wsl); err != nil {
				fmt.Printf("         [FAIL] mkdir failed: %v\n", err)
				fail = true
				continue
			}
			if err := wsl("cp -f " + wslSrc + "/aly-client.exe " + t.wsl + "/"); err != nil {
				fmt.Printf("         [FAIL] copy failed: %v\n", err)
				fail = true
				continue
			}
			out, err := wslOutput("stat -c%s " + t.wsl + "/aly-client.exe")
			if err == nil {
				var size int64
				fmt.Sscanf(out, "%d", &size)
				fmt.Printf("         OK (%.1f MB)\n", float64(size)/1024/1024)
			} else {
				fmt.Println("         OK")
			}
		}
		if fail {
			os.Exit(1)
		}
		fmt.Println()
	}

	// ── Step 4: Clean ──
	fmt.Println("  [4/4] Clean WSL ...")
	if err := wsl("rm -rf " + wslSrc); err != nil {
		fmt.Printf("  [WARN] clean failed: %v\n", err)
	}
	fmt.Println("  Done\n")

	fmt.Println(strings.Repeat("=", 50))
	if len(targets) > 0 {
		for _, t := range targets {
			fmt.Printf("  Deployed: %s\\aly-client.exe\n", t.win)
		}
	} else {
		fmt.Println("  No targets deployed (check config.json)")
	}
	fmt.Println(strings.Repeat("=", 50))
}
