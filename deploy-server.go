// deploy-server: 交叉编译 aly-server-linux-amd64 → SSH 部署到远程 Linux 服务器
//
// 用法: go run deploy-server.go [password]
//   如果不传密码参数，会从 STDIN 或 SSH_ASKPASS 获取
// 前置: 本机已安装 Go + OpenSSH 客户端

package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
)

const (
	host      = "10.96.115.14"
	user      = "quartecs"
	remoteDir = "/home/quartecs/lishuangquan/aly"
	binary    = "aly-server-linux-amd64"
	port      = "7000"
)

var (
	scriptDir string
	serverDir string
	localPath string
)

func mustInit() {
	var err error
	scriptDir, err = os.Getwd()
	if err != nil {
		scriptDir = "."
	}
	scriptDir, _ = filepath.Abs(scriptDir)
	serverDir = filepath.Join(scriptDir, "server")
	localPath = filepath.Join(serverDir, binary)
}

// runCmd runs a command, streaming stdout/stderr to the console.
func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runCmdInDir runs a command in the given directory.
func runCmdInDir(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ── SSH with password ────────────────────────

func sshWithPassword(password string, args ...string) error {
	// Prefer sshpass if available
	if _, err := exec.LookPath("sshpass"); err == nil {
		allArgs := append([]string{"-p", password, "ssh"}, args...)
		allArgs = append(allArgs, "-o", "StrictHostKeyChecking=no")
		return runCmd("sshpass", allArgs...)
	}

	// Fallback: SSH_ASKPASS with temporary script
	askpassFile, cleanup, err := createAskPass(password)
	if err != nil {
		return fmt.Errorf("SSH_ASKPASS setup failed: %w", err)
	}
	defer cleanup()

	env := append(os.Environ(),
		"SSH_ASKPASS="+askpassFile,
		"SSH_ASKPASS_REQUIRE=force",
		"DISPLAY=dummy",
	)

	allArgs := append([]string{"ssh", "-o", "StrictHostKeyChecking=no", "-o", "BatchMode=no"}, args...)
	cmd := exec.Command(allArgs[0], allArgs[1:]...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func scpWithPassword(password, src, dst string) error {
	// Prefer sshpass if available
	if _, err := exec.LookPath("sshpass"); err == nil {
		return runCmd("sshpass", "-p", password, "scp",
			"-o", "StrictHostKeyChecking=no", src, dst)
	}

	// Fallback: SSH_ASKPASS
	askpassFile, cleanup, err := createAskPass(password)
	if err != nil {
		return fmt.Errorf("SSH_ASKPASS setup failed: %w", err)
	}
	defer cleanup()

	env := append(os.Environ(),
		"SSH_ASKPASS="+askpassFile,
		"SSH_ASKPASS_REQUIRE=force",
		"DISPLAY=dummy",
	)

	cmd := exec.Command("scp", "-o", "StrictHostKeyChecking=no", src, dst)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// createAskPass writes a temporary .bat script that echoes the password.
// Returns (path, cleanup, error).
func createAskPass(password string) (string, func(), error) {
	f, err := os.CreateTemp("", "sshpass-*.bat")
	if err != nil {
		return "", nil, err
	}
	// Windows batch: @echo off + echo password
	content := "@echo off\r\necho " + password + "\r\n"
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil, err
	}
	f.Close()
	cleanup := func() { os.Remove(f.Name()) }
	return f.Name(), cleanup, nil
}

// getPassword reads password from arg or stdin.
func getPassword() (string, error) {
	if len(os.Args) > 1 {
		return os.Args[1], nil
	}

	fmt.Printf("Password for %s@%s: ", user, host)
	reader := bufio.NewReader(os.Stdin)
	pass, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(pass, "\r\n"), nil
}

func main() {
	mustInit()
	password, err := getPassword()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read password: %v\n", err)
		os.Exit(1)
	}

	// ── Step 1: Build ────────────────────────
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("  [1/5] Building %s (linux/amd64) ...\n", binary)
	env := append(os.Environ(), "GOOS=linux", "GOARCH=amd64")
	cmd := exec.Command("go", "build", "-o", binary, ".")
	cmd.Dir = serverDir
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("[ERROR] Build failed: %v\n", err)
		os.Exit(1)
	}
	info, _ := os.Stat(localPath)
	sizeMB := float64(info.Size()) / 1024 / 1024
	fmt.Printf("       Build OK (%.1f MB)\n\n", sizeMB)

	// Ignore SIGINT during SSH operations so Ctrl+C doesn't leave orphaned connections
	signal.Ignore(os.Interrupt)
	defer signal.Reset(os.Interrupt)

	// ── Step 2: Kill old process ─────────────
	fmt.Printf("[2/5] Stopping port %s ...\n", port)
	stopCmd := fmt.Sprintf(
		"pids=$(lsof -ti :%s); "+
			"if [ -n \"$pids\" ]; then "+
			"  kill -TERM $pids 2>/dev/null; "+
			"  for i in $(seq 1 5); do "+
			"    sleep 1; "+
			"    if ! kill -0 $pids 2>/dev/null; then break; fi; "+
			"  done; "+
			"  kill -9 $pids 2>/dev/null; "+
			"fi; "+
			"sleep 1; echo OK",
		port,
	)
	if err := sshWithPassword(password, user+"@"+host, stopCmd); err != nil {
		fmt.Printf("[WARN] Stop command had issues: %v\n", err)
	}
	fmt.Println("       done\n")

	// ── Step 3: Upload ───────────────────────
	fmt.Println("[3/5] Uploading ...")
	if err := scpWithPassword(password, localPath,
		fmt.Sprintf("%s@%s:%s/%s", user, host, remoteDir, binary)); err != nil {
		fmt.Printf("[ERROR] Upload failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("       Upload OK\n")

	// ── Step 4: chmod ────────────────────────
	fmt.Println("[4/5] chmod +x ...")
	if err := sshWithPassword(password, user+"@"+host,
		"chmod +x "+remoteDir+"/"+binary); err != nil {
		fmt.Printf("[ERROR] chmod failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("       done\n")

	// ── Step 5: Start (detached) ─────────────
	fmt.Println("[5/5] Starting server (detached) ...")
	startCmd := fmt.Sprintf(
		"cd %s && setsid ./%s -p %s </dev/null >>nohup.out 2>&1 &",
		remoteDir, binary, port,
	)
	if err := sshWithPassword(password, user+"@"+host, startCmd); err != nil {
		fmt.Printf("[ERROR] Start failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("       done\n")

	// ── Verify ───────────────────────────────
	verifyCmd := fmt.Sprintf("lsof -i :%s 2>/dev/null | head -2", port)
	// For verification, we just run a simple ssh command
	verify := exec.Command("sshpass", "-p", password, "ssh",
		"-o", "StrictHostKeyChecking=no",
		user+"@"+host, verifyCmd)
	verify.Stdout = os.Stdout
	verify.Stderr = os.Stderr

	// Fallback to SSH_ASKPASS if sshpass not available
	if _, err := exec.LookPath("sshpass"); err != nil {
		askpassFile, cleanup, err := createAskPass(password)
		if err == nil {
			defer cleanup()
			verify = exec.Command("ssh",
				"-o", "StrictHostKeyChecking=no",
				"-o", "BatchMode=no",
				user+"@"+host, verifyCmd)
			verify.Env = append(os.Environ(),
				"SSH_ASKPASS="+askpassFile,
				"SSH_ASKPASS_REQUIRE=force",
				"DISPLAY=dummy",
			)
		}
	}
	// Best effort verify — don't fail on verify error
	_ = verify.Run()

	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("  Deploy success! Check port 7000 manually if needed.")
	fmt.Println(strings.Repeat("=", 50))
}
