// deploy-client: 在 WSL 内用 Docker 镜像编译 aly-client.exe 与 aly-client-ui.exe
// （windows/386，Windows XP 兼容），校验 PE 头后回拷仓库并部署到 config.json 目标目录。
//
// 用法: go run deploy-client.go
// 前置: Windows 已安装 WSL；WSL 内 docker 可用；编译镜像已存在（默认
//
//	golang:1.10.8-stretch，可用 ALY_GO_IMAGE 覆盖）。
//
// 为什么用容器而不是 WSL 内装的 Go：XP 兼容要求 Go 1.10.x + GOPATH 模式 +
// GOARCH=386（见 client/aly-client/BUILD.md），这个约束现在由镜像 tag 固定；
// 容器同时保证工具链、GOPATH 布局与依赖（仓库内 vendor）每次完全一致，
// 不再依赖宿主机 WSL 里的 Go 安装。
//
// 环境变量（均可省略）：
//
//	ALY_GO_IMAGE   编译镜像，默认 golang:1.10.8-stretch，必须是 go1.10.x
//	ALY_BUILD_DIR  临时产物目录的父目录，默认系统 TEMP；
//	               必须是 Docker 能挂载的盘符路径（不能是 WSL 内部路径）
package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	defaultGoImage = "golang:1.10.8-stretch"

	// 容器内路径：源码只读挂在 GOPATH 下，产物写 /out。
	containerSrcRoot = "/go/src/aly/client"
	containerWorkDir = containerSrcRoot + "/aly-client"
	containerOutDir  = "/out"

	// 构建环境：Windows XP 的硬性口径（见 client/aly-client/BUILD.md）。
	//   GOOS=windows GOARCH=386 —— 32 位，XP 没有 64 位 Go 运行时
	//   CGO_ENABLED=0           —— 交叉编译，不依赖目标机 C 运行库
	//   HOME/GOCACHE 指到 /tmp  —— 容器默认 root 运行，构建缓存只落容器内 /tmp，
	//                              不污染挂载目录（产物才写 /out）
	//   GOPATH=/go              —— 镜像默认值，显式写出避免歧义
	//
	// 依赖已 vendor，因此构建用 --network none 离线执行：既保证可复现，
	// 也避免 GOPATH 模式静默去拉依赖仓库 HEAD（曾导致零改动编译失败）。
	buildEnvArgs = "-e HOME=/tmp -e GOCACHE=/tmp/gocache " +
		"-e GOOS=windows -e GOARCH=386 -e CGO_ENABLED=0 -e GOPATH=/go"

	// PE 头口径：Machine=0x014C(386) / Magic=0x010B(PE32) /
	// Subsystem 版本钉死 Go 1.10 的 4.0（Windows XP 加载器上限是 5.1，
	// 现代 Go 产出 6.x，均不可用）。
	peMachine386          = 0x014C
	peMagicPE32           = 0x010B
	peExpectedMajorSub    = 4
	peExpectedMinorSub    = 0
	peXPMajorSubsystemVer = 5
	peXPMinorSubsystemVer = 1

	// PE Subsystem：2=WINDOWS_GUI（无控制台），3=CONSOLE。
	peSubsystemGUI     = 2
	peSubsystemConsole = 3
)

// artifact 是一个必须同进同退的产物：构建、校验、回拷、部署走同一张表，
// 避免出现"更新器有新功能、UI 还是旧版"的错位。
type artifact struct {
	name      string // 产物文件名（容器 /out 与仓库内同名）
	repoRel   string // 相对 client/ 的仓库落地位置（自更新的比对源）
	pkg       string // GOPATH 模式下的包参数
	ldflags   string // -ldflags 值
	subsystem uint16 // 期望的 PE Subsystem
}

var artifacts = []artifact{
	{
		name:      "aly-client.exe",
		repoRel:   "aly-client/aly-client.exe",
		pkg:       ".",
		ldflags:   "-s -w",
		subsystem: peSubsystemConsole,
	},
	{
		name:      "aly-client-ui.exe",
		repoRel:   "aly-client-ui/aly-client-winui/aly-client-ui.exe",
		pkg:       "aly/client/aly-client-ui/aly-client-winui",
		ldflags:   "-s -w -H=windowsgui",
		subsystem: peSubsystemGUI,
	},
}

var (
	scriptDir  string // 仓库根（Windows 绝对路径）
	repoClient string // 仓库 client 目录（Windows）
	outDir     string // 临时产物目录（Windows，Docker 可挂载的盘符路径）
	goImage    string
)

// envOrDefault 读环境变量，空则返回默认值（不硬编码个人路径）。
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// requireDrivePath 断言 p 是盘符绝对路径：Docker 只能挂载 /mnt/<盘符>，
// WSL 内部路径与 UNC 路径都会被拼成不可用（甚至静默为空）的挂载。
func requireDrivePath(p, label string) error {
	if len(p) < 3 || p[1] != ':' {
		return fmt.Errorf("%s 必须是盘符绝对路径（Docker 只能挂载 /mnt/<盘符>）：%s", label, p)
	}
	return nil
}

// mustInit 解析仓库根并创建临时产物目录。
func mustInit() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("读取当前目录失败: %v", err)
	}
	scriptDir, err = filepath.Abs(wd)
	if err != nil {
		return fmt.Errorf("解析仓库根路径失败: %v", err)
	}

	// 本脚本位于仓库 deploy/ 子目录：当前目录找不到 client/aly-client 时
	// 回退一级到仓库根（保持从任意工作目录运行均可用）。
	if _, statErr := os.Stat(filepath.Join(scriptDir, "client", "aly-client")); os.IsNotExist(statErr) {
		parent := filepath.Dir(scriptDir)
		if _, pErr := os.Stat(filepath.Join(parent, "client", "aly-client")); pErr == nil {
			scriptDir = parent
		}
	}

	goImage = envOrDefault("ALY_GO_IMAGE", defaultGoImage)
	repoClient = filepath.Join(scriptDir, "client")

	// 临时产物目录必须落在宿主的盘符路径上：Docker 只能挂载 /mnt/<盘符>，
	// 看不到 WSL 内部路径（/home、/tmp 等）。
	buildBase := envOrDefault("ALY_BUILD_DIR", "")
	if buildBase != "" {
		if buildBase, err = filepath.Abs(buildBase); err != nil {
			return fmt.Errorf("ALY_BUILD_DIR 无效: %v", err)
		}
		if err := requireDrivePath(buildBase, "ALY_BUILD_DIR"); err != nil {
			return err
		}
	}
	if outDir, err = os.MkdirTemp(buildBase, "aly-client-build-"); err != nil {
		return fmt.Errorf("创建临时产物目录失败: %v", err)
	}
	// 默认 TEMP 被重定向到 UNC/\\wsl$ 时同样挂不上去，这里一并拦住。
	if err := requireDrivePath(outDir, "临时产物目录"); err != nil {
		os.RemoveAll(outDir)
		outDir = ""
		return err
	}
	return nil
}

// wsl 在默认 WSL 发行版里执行一条 bash 命令，输出直通终端。
func wsl(cmd string) error {
	c := exec.Command("wsl", "bash", "-c", cmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// wslOutput 执行命令并取回 stdout（stderr 仍直通终端）。
func wslOutput(cmd string) (string, error) {
	c := exec.Command("wsl", "bash", "-c", cmd)
	c.Stderr = os.Stderr
	out, err := c.Output()
	return strings.TrimSpace(string(out)), err
}

// winToWsl 把 Windows 盘符路径转换成 WSL/Docker 能识别的 /mnt/<盘符> 路径。
func winToWsl(winPath string) string {
	if len(winPath) < 2 {
		return winPath
	}
	drive := strings.ToLower(string(winPath[0]))
	rest := strings.ReplaceAll(winPath[2:], "\\", "/")
	return "/mnt/" + drive + rest
}

// shQuote 把任意字符串安全地嵌进 bash 命令串（审查 M29）。
//
// 路径里的空格会被 word-split，`;`、`$()`、反引号会被当成命令执行；
// 单引号包裹，内部单引号按 POSIX 约定「闭合-转义-重开」，是通用安全写法。
func shQuote(s string) string {
	return "'" + strings.Replace(s, "'", `'\''`, -1) + "'"
}

// preflight 在动任何产物之前确认能力齐备：路径、docker、镜像、镜像内 Go 版本。
func preflight() error {
	if err := requireDrivePath(scriptDir, "仓库路径"); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(repoClient, "aly-client")); err != nil {
		return fmt.Errorf("找不到源码目录: %s", filepath.Join(repoClient, "aly-client"))
	}

	if _, err := wslOutput("docker version --format '{{.Server.Version}}'"); err != nil {
		return fmt.Errorf("WSL 内 docker 不可用（确认已安装 WSL、启动 Docker 并开启 WSL 集成）: %v", err)
	}

	if _, err := wslOutput("docker image inspect " + shQuote(goImage) + " --format '{{.Id}}'"); err != nil {
		return fmt.Errorf("镜像不存在: %s\n"+
			"         请先执行: wsl docker pull %s\n"+
			"         或把 ALY_GO_IMAGE 指到本机已有的 1.10.x 镜像副本", goImage, goImage)
	}

	goVer, err := wslOutput("docker run --rm --network none " + shQuote(goImage) + " go version")
	if err != nil {
		return fmt.Errorf("镜像内无法运行 go: %v", err)
	}
	// XP 只接受 Go 1.10 产出的二进制（现代 Go 的 PE SubsystemVer 是 6.x）。
	// 版本不对必须在构建前拦住，否则会静默产出 XP 加载不了的 exe。
	if !strings.HasPrefix(goVer, "go version go1.10.") {
		return fmt.Errorf("镜像内的 Go 版本不是 go1.10.x（实际: %s）；"+
			"Windows XP 只接受 Go 1.10 产物（见 client/aly-client/BUILD.md）", goVer)
	}

	fmt.Printf("         image=%s (%s)\n", goImage, goVer)
	return nil
}

// buildArtifact 在容器里编译单个产物，输出到临时目录。
func buildArtifact(a artifact) error {
	cmd := "docker run --rm --network none" +
		" " + buildEnvArgs +
		" -w " + containerWorkDir +
		// 源码只读挂载：容器内不产生任何对仓库的写入（否则 Go 可能留下缓存/中间文件）
		" -v " + shQuote(winToWsl(repoClient)+":"+containerSrcRoot+":ro") +
		" -v " + shQuote(winToWsl(outDir)+":"+containerOutDir) +
		" " + shQuote(goImage) +
		" go build -ldflags=" + shQuote(a.ldflags) +
		" -o " + shQuote(containerOutDir+"/"+a.name) +
		" " + shQuote(a.pkg)
	return wsl(cmd)
}

// buildAll 依次构建所有产物；任一失败即中止（不允许"只拷一半"）。
func buildAll() error {
	for _, a := range artifacts {
		fmt.Printf("         building %s ...\n", a.name)
		if err := buildArtifact(a); err != nil {
			return fmt.Errorf("构建 %s 失败: %v", a.name, err)
		}
	}
	return nil
}

// peInfo 是校验产物所需的 PE 头字段。
type peInfo struct {
	machine   uint16
	magic     uint16
	majorSub  uint16
	minorSub  uint16
	subsystem uint16
}

func readU16At(f *os.File, off int64) (uint16, error) {
	b := make([]byte, 2)
	if _, err := f.ReadAt(b, off); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(b), nil
}

func readU32At(f *os.File, off int64) (uint32, error) {
	b := make([]byte, 4)
	if _, err := f.ReadAt(b, off); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(b), nil
}

// verifyPE 读取 PE 头（DOS 头 -> e_lfanew -> COFF/可选头）。
func verifyPE(path string) (peInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return peInfo{}, err
	}
	defer f.Close()

	sig := make([]byte, 2)
	if _, err := f.ReadAt(sig, 0); err != nil {
		return peInfo{}, err
	}
	if sig[0] != 'M' || sig[1] != 'Z' {
		return peInfo{}, fmt.Errorf("不是 PE 文件（缺少 MZ 头）")
	}

	peOff, err := readU32At(f, 0x3C)
	if err != nil {
		return peInfo{}, err
	}
	off := int64(peOff)

	peSig := make([]byte, 4)
	if _, err := f.ReadAt(peSig, off); err != nil {
		return peInfo{}, err
	}
	if string(peSig) != "PE\x00\x00" {
		return peInfo{}, fmt.Errorf("不是 PE 文件（缺少 PE 签名）")
	}

	var info peInfo
	if info.machine, err = readU16At(f, off+4); err != nil {
		return peInfo{}, err
	}
	// 可选头紧接 COFF 头（20 字节）；其长度在 COFF 头 +16 处。
	sizeOfOpt, err := readU16At(f, off+20)
	if err != nil {
		return peInfo{}, err
	}
	if sizeOfOpt < 70 {
		return peInfo{}, fmt.Errorf("可选头长度异常（SizeOfOptionalHeader=%d）", sizeOfOpt)
	}
	// 可选头从 COFF 头（20 字节）之后开始。
	optOff := off + 24
	if info.magic, err = readU16At(f, optOff); err != nil {
		return peInfo{}, err
	}
	// PE32 标准字段：Magic(2) + ... ; MajorSubsystemVersion 在可选头 +48。
	if info.majorSub, err = readU16At(f, optOff+48); err != nil {
		return peInfo{}, err
	}
	if info.minorSub, err = readU16At(f, optOff+50); err != nil {
		return peInfo{}, err
	}
	if info.subsystem, err = readU16At(f, optOff+68); err != nil {
		return peInfo{}, err
	}
	return info, nil
}

// verifyArtifacts 校验两个产物确实是 32 位、XP 可加载的对应形态。
func verifyArtifacts() error {
	for _, a := range artifacts {
		path := filepath.Join(outDir, a.name)
		st, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("产物缺失: %s: %v", a.name, err)
		}
		if st.Size() < 64<<10 {
			return fmt.Errorf("%s 体积异常（%d 字节），构建可能未完成", a.name, st.Size())
		}
		info, err := verifyPE(path)
		if err != nil {
			return fmt.Errorf("读取 %s 的 PE 头失败: %v", a.name, err)
		}
		if info.machine != peMachine386 {
			return fmt.Errorf("%s 不是 32 位产物（Machine=0x%04X）", a.name, info.machine)
		}
		if info.magic != peMagicPE32 {
			return fmt.Errorf("%s 不是 PE32（Magic=0x%04X）", a.name, info.magic)
		}
		// Go 1.10 固定产出 4.0；XP 加载器上限是 5.1，现代 Go 是 6.x。
		// 直接钉死 4.0：这是"必须用 Go 1.10 构建"的产物级最后一道防线。
		if info.majorSub != peExpectedMajorSub || info.minorSub != peExpectedMinorSub {
			return fmt.Errorf("%s 的 PE SubsystemVer=%d.%d，期望 %d.%d（Go 1.10 的产物；"+
				"Windows XP 加载器上限是 %d.%d）",
				a.name, info.majorSub, info.minorSub, peExpectedMajorSub, peExpectedMinorSub,
				peXPMajorSubsystemVer, peXPMinorSubsystemVer)
		}
		if info.subsystem != a.subsystem {
			return fmt.Errorf("%s 的 PE Subsystem=%d，期望 %d", a.name, info.subsystem, a.subsystem)
		}
		fmt.Printf("         OK %s (386, PE32, SubsystemVer=%d.%d, Subsystem=%d, %.1fMB)\n",
			a.name, info.majorSub, info.minorSub, info.subsystem, float64(st.Size())/1024/1024)
	}
	return nil
}

// filePair 是一次「源 -> 目标」发布。
type filePair struct {
	src string
	dst string
}

// resolveDst 解析目标的真实落点，并拦掉明显不可替换的目标：
//   - 符号链接：与旧版 `cp -f` 一致，写到链接指向的真实文件
//   - 目录：直接报错（不能把目录静默替换成 exe）
func resolveDst(dst string) (string, error) {
	li, err := os.Lstat(dst)
	if os.IsNotExist(err) {
		return dst, nil
	}
	if err != nil {
		return "", err
	}
	if li.IsDir() {
		return "", fmt.Errorf("%s 是目录，拒绝覆盖", dst)
	}
	if li.Mode()&os.ModeSymlink != 0 {
		real, err := filepath.EvalSymlinks(dst)
		if err != nil {
			return "", fmt.Errorf("解析符号链接 %s 失败: %v", dst, err)
		}
		// 链接指向的也必须是可替换的普通文件（可能是目录/只读）。
		st, err := os.Stat(real)
		if err != nil {
			return "", fmt.Errorf("读取链接目标 %s 失败: %v", real, err)
		}
		if st.IsDir() {
			return "", fmt.Errorf("%s 指向目录 %s，拒绝覆盖", dst, real)
		}
		return real, nil
	}
	return dst, nil
}

// stageFile 把 src 写成 dst 同目录下的临时文件，返回（临时路径, 最终路径）。
//
// 只写临时文件、不碰旧文件：任何一步失败（源读不到、磁盘满、Sync 失败）
// 都能让旧产物原样保留。落在同目录是为了让后续 Rename 成为同一卷内的原子替换。
func stageFile(src, dst string) (string, string, error) {
	finalDst, err := resolveDst(dst)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(filepath.Dir(finalDst), 0755); err != nil {
		return "", "", err
	}

	in, err := os.Open(src)
	if err != nil {
		return "", "", err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return "", "", err
	}

	// 临时文件用唯一名（os.CreateTemp）：并发跑两个实例时不会互相踩同一个
	// 临时文件；同目录创建保证后续 Rename 仍是同卷原子替换。
	out, err := os.CreateTemp(filepath.Dir(finalDst), filepath.Base(finalDst)+"-*.tmp")
	if err != nil {
		return "", "", err
	}
	tmp := out.Name()

	n, copyErr := io.Copy(out, in)
	if copyErr == nil && n != st.Size() {
		copyErr = fmt.Errorf("写入 %d 字节，与源文件 %d 字节不一致", n, st.Size())
	}
	if copyErr == nil {
		// 落盘后再返回：这份 exe 会被自更新当作比对源，半截文件比"没拷贝"更难排查。
		copyErr = out.Sync()
	}
	if closeErr := out.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		os.Remove(tmp)
		return "", "", copyErr
	}
	return tmp, finalDst, nil
}

// publishSet 发布一组产物：先全部暂存成功，再确认所有目标可写，最后逐个原子替换
// （os.Rename 在 Windows 上是 MoveFileEx(REPLACE_EXISTING)）。
//
// 三个阶段的意义：更新器与 UI 必须同进同退。任一产物写不出来、或任一目标被占用
// （exe 正在运行）时，临时文件全部清掉、旧文件一个都不动；避免"旧的被删掉、
// 新的只写了一半"，也尽量缩小"新更新器 + 旧 UI"的窗口。
func publishSet(pairs []filePair) error {
	tmps := make([]string, len(pairs))
	dsts := make([]string, len(pairs))
	for i, p := range pairs {
		tmp, finalDst, err := stageFile(p.src, p.dst)
		if err != nil {
			for _, t := range tmps[:i] {
				os.Remove(t)
			}
			return fmt.Errorf("%s: %v", filepath.Base(p.dst), err)
		}
		tmps[i], dsts[i] = tmp, finalDst
	}

	// 提交前先探测目标可写性（并按需清掉只读属性）：被运行中的客户端占用的
	// exe 在这里就会失败，旧文件与旧状态都不动。注意"能写"不等于"能替换"，
	// 真正的替换窗口仍在下面两次 Rename 之间（同目录同 ACL，窗口极小）。
	for i := range dsts {
		st, err := os.Stat(dsts[i])
		if os.IsNotExist(err) {
			continue // 新目标，由 Rename 创建
		}
		if err != nil {
			for _, t := range tmps {
				os.Remove(t)
			}
			return fmt.Errorf("%s: %v", filepath.Base(dsts[i]), err)
		}
		// 只读文件既不能覆盖也不能被 MoveFileEx 替换，先清只读位；
		// 放在提交前而不是暂存时，保证"暂存失败则旧文件属性也不变"。
		if st.Mode().Perm()&0200 == 0 {
			if err := os.Chmod(dsts[i], 0644); err != nil {
				for _, t := range tmps {
					os.Remove(t)
				}
				return fmt.Errorf("%s: 清除只读属性失败: %v", filepath.Base(dsts[i]), err)
			}
		}
		f, err := os.OpenFile(dsts[i], os.O_WRONLY, 0)
		if err != nil {
			for _, t := range tmps {
				os.Remove(t)
			}
			return fmt.Errorf("%s: 目标不可写（可能正在运行）: %v", filepath.Base(dsts[i]), err)
		}
		f.Close()
	}

	// 逐个原子替换：Rename 在 Windows 上是 MoveFileEx(REPLACE_EXISTING)。
	// 正常路径下两件套要么都换、要么都不换；只有"第一个已替换、第二个被拒"
	// 这种极小概率会留下新旧混搭（重跑一次即可收敛）。
	for i := range tmps {
		if err := os.Rename(tmps[i], dsts[i]); err != nil {
			for _, t := range tmps[i:] {
				os.Remove(t)
			}
			return fmt.Errorf("%s: %v", filepath.Base(dsts[i]), err)
		}
	}
	return nil
}

// repoDst 返回产物在仓库中的落地位置（自更新的比对源）。
func repoDst(a artifact) string {
	return filepath.Join(repoClient, a.repoRel)
}

// copyBackToRepo 把通过校验的产物回拷仓库；两件套一起成功或一起不动。
func copyBackToRepo() error {
	pairs := make([]filePair, 0, len(artifacts))
	for _, a := range artifacts {
		pairs = append(pairs, filePair{src: filepath.Join(outDir, a.name), dst: repoDst(a)})
	}
	if err := publishSet(pairs); err != nil {
		return fmt.Errorf("回拷仓库失败: %v", err)
	}
	for _, p := range pairs {
		fmt.Printf("         OK %s\n", p.dst)
	}
	return nil
}

func configPath() string {
	appdata := os.Getenv("LOCALAPPDATA")
	if appdata == "" {
		home, _ := os.UserHomeDir()
		appdata = filepath.Join(home, "AppData", "Local")
	}
	return filepath.Join(appdata, "AlyPublish", "config.json")
}

type projectConfig struct {
	ProjectPath string `json:"ProjectPath"`
}

// loadDeployTargets 从 config.json 推导部署目录：项目主目录 + 同级 UpdateFolder。
func loadDeployTargets() []string {
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

	targets := []string{}
	seen := make(map[string]bool)

	for _, proj := range projects {
		pp := proj.ProjectPath
		if pp == "" || !filepath.IsAbs(pp) {
			continue
		}
		pp = filepath.Clean(pp)

		// Target 1: ProjectPath itself（Windows 路径大小写不敏感，去重键统一小写）
		if key := strings.ToLower(pp); !seen[key] {
			seen[key] = true
			targets = append(targets, pp)
		}

		// Target 2: ProjectPath/../UpdateFolder
		updateFolder := filepath.Clean(filepath.Join(pp, "..", "UpdateFolder"))
		if key := strings.ToLower(updateFolder); !seen[key] {
			seen[key] = true
			targets = append(targets, updateFolder)
		}
	}
	return targets
}

// deployPairs 返回把全部产物发布到某个目录的「源 -> 目标」列表。
func deployPairs(dir string) []filePair {
	pairs := make([]filePair, 0, len(artifacts))
	for _, a := range artifacts {
		pairs = append(pairs, filePair{
			src: filepath.Join(outDir, a.name),
			dst: filepath.Join(dir, a.name),
		})
	}
	return pairs
}

// deploy 把两个产物部署到所有目标；单个目标失败不阻断其余目标，最后统一报错。
func deploy(targets []string) error {
	failed := 0
	for i, dir := range targets {
		fmt.Printf("         [%d/%d] %s\n", i+1, len(targets), dir)
		// 一个目标的两件套要么都换、要么都留旧：publishSet 先写临时文件再统一替换。
		if err := publishSet(deployPairs(dir)); err != nil {
			fmt.Printf("         [FAIL] %v\n", err)
			failed++
			continue
		}
		sizes := ""
		sizeOK := true
		for _, a := range artifacts {
			dst := filepath.Join(dir, a.name)
			st, err := os.Stat(dst)
			if err != nil {
				fmt.Printf("         [FAIL] stat %s failed: %v\n", a.name, err)
				sizeOK = false
				break
			}
			sizes += fmt.Sprintf("%s=%.1fMB ", a.name, float64(st.Size())/1024/1024)
		}
		if !sizeOK {
			failed++
			continue
		}
		fmt.Printf("         OK (%s)\n", strings.TrimSpace(sizes))
	}
	if failed > 0 {
		return fmt.Errorf("%d/%d 个部署目标失败（详见上方 [FAIL]）", failed, len(targets))
	}
	return nil
}

func run() error {
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("  aly-client XP build (%s, windows/386)\n", goImage)

	fmt.Println("  [1/5] Preflight (path / docker / image / go version) ...")
	if err := preflight(); err != nil {
		return err
	}
	fmt.Print("  Done\n\n")

	fmt.Println("  [2/5] Building aly-client.exe + aly-client-ui.exe ...")
	if err := buildAll(); err != nil {
		return err
	}
	fmt.Print("  Build OK\n\n")

	fmt.Println("  [3/5] Verifying XP artifacts (PE header) ...")
	if err := verifyArtifacts(); err != nil {
		return err
	}
	fmt.Print("  Artifacts OK\n\n")

	fmt.Println("  [4/5] Copying artifacts back to repo ...")
	if err := copyBackToRepo(); err != nil {
		return err
	}
	fmt.Println()

	targets := loadDeployTargets()
	if len(targets) > 0 {
		fmt.Printf("  [5/5] Deploying to %d target(s) ...\n", len(targets))
		if err := deploy(targets); err != nil {
			return err
		}
		fmt.Println()
	} else {
		fmt.Println("  [5/5] No deploy targets found in config.json (only repo copy-back)")
	}

	fmt.Println(strings.Repeat("=", 50))
	if len(targets) > 0 {
		for _, dir := range targets {
			for _, a := range artifacts {
				fmt.Printf("  Deployed: %s\\%s\n", dir, a.name)
			}
		}
	} else {
		fmt.Println("  No targets deployed (check config.json)")
	}
	fmt.Println(strings.Repeat("=", 50))
	return nil
}

func main() {
	if err := mustInit(); err != nil {
		fmt.Printf("  [ERROR] %v\n", err)
		os.Exit(1)
	}

	err := run()

	// 临时目录只存本次产物：无论成败都清理（清理失败不改变退出码，
	// 但产物可能已被占用，留一条 WARN 便于排查）。
	if outDir != "" {
		if rmErr := os.RemoveAll(outDir); rmErr != nil {
			fmt.Printf("  [WARN] clean temp dir failed: %v\n", rmErr)
		}
	}

	if err != nil {
		fmt.Printf("  [ERROR] %v\n", err)
		os.Exit(1)
	}
}
