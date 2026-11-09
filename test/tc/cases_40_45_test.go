package tc

// 用例 40–45：aly-client-ui（winui）
//
// 这一批的核心约束：UI 是 windowsgui 子系统，**没有可靠的 stdout**，
// 也无法在自动化里点按钮。因此策略是分两层：
//   ① 能用构建产物 / 单测证明的，全部走 PE 头 + GOPATH 模式 go test；
//   ② 真实的 -mode 流程只断言"进程生命周期与磁盘副作用"（是否退出、版本目录有没有出现、
//      version.json 有没有被越权改写），不对像素或弹窗文案做断言。

import (
	"path/filepath"
	"testing"
	"time"
)

// ────────────────────────────────────────────────────────────────
// 用例 40 — UI 构建（386 + windowsgui）
// ────────────────────────────────────────────────────────────────
func TestCase40(t *testing.T) {
	e := NewEnv(t, true, 0)

	uiExe := e.BuildUI(t)

	True(t, Exists(uiExe), "UI 产物缺失: %s", uiExe)
	size := SizeOf(uiExe)
	True(t, size > 1<<20, "UI 体积异常: %d 字节（walk 静态链接通常 >4MiB）", size)
	Eq(t, "PE 架构（必须 i386，否则 XP 跑不起来）", PEMachine(uiExe), MachineI386)
	Eq(t, "PE 子系统（必须 WINDOWS_GUI，否则双击弹黑框）",
		PESubsystem(uiExe), SubsystemWindowsGUI)

	t.Logf("UI 产物 %s，%d 字节，i386/WINDOWS_GUI", filepath.Base(uiExe), size)
}

// ────────────────────────────────────────────────────────────────
// 用例 41 — UI bridge 单测全绿
//
// 断言清单以仓库里的真实函数名为准（文档里的旧清单漏了第四批新增的
// TestParseLineForceUpdateAbsent / TestParseLineRollbackResult 等）。
// ────────────────────────────────────────────────────────────────
func TestCase41(t *testing.T) {
	e := NewEnv(t, true, 0)

	code, out := e.GoTestGopath(t, "aly/client/aly-client-ui/aly-client-winui/bridge",
		"-v", "-count=1")
	Eq(t, "bridge 单测退出码", code, 0)
	NotContains(t, out, "--- FAIL", "bridge 单测不应有失败项")

	// 这 11 条是超时/取消/解析语义的锚点：缺任何一条都意味着
	// "90s 停流判定"或"暂停即 cancel"这条规则没被真正测到。
	for _, name := range []string{
		"TestRunMissingExe",                // 找不到 exe 时快速失败，不挂死
		"TestRunParsesDoneLine",            // DONE 行（data:null）解析
		"TestRunParsesResultAndSkipsNoise", // 噪声行跳过，只认结果行
		"TestRunProgressCallback",          // PROGRESS 回调被逐条触发
		"TestRunTotalTimeout",              // quickTimeout 生效
		"TestRunIdleTimeout",               // idleTimeout 生效（下载停流才判死）
		"TestRunContextCancel",             // 暂停＝cancel 子进程
		"TestParseLineProgress",            // PROGRESS 字段映射
		"TestParseLineProgressDone",        // 完成行 data:null
		"TestParseLineProgressFail",        // FAIL 行 error 字段
		"TestParseLineRollbackList",        // 回滚列表
	} {
		Contains(t, out, "--- PASS: "+name, "bridge 单测缺 PASS")
	}

	// 第四批新增：force_update 缺省必须是 nil（续跑分支不返回该字段），
	// 若被解析成 false 会让 UI 把"强制更新"误判成"可选"。
	Contains(t, out, "--- PASS: TestParseLineForceUpdateAbsent", "force_update 三态")

	if n := countOccurrences(out, "--- SKIP"); n > 0 {
		t.Logf("注意：bridge 有 %d 条 SKIP（integration_test.go 依赖真实 exe），SKIP 不算失败", n)
	}
}

// countOccurrences 统计子串出现次数（上报 SKIP 计数用）。
func countOccurrences(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

// ────────────────────────────────────────────────────────────────
// 用例 42 — UI `-mode check` 无更新：提示后干净退出
//
// 修正后的真实期望（与 window.go:263 的设计一致）：
//   无更新时 UI 先弹一个**阻塞式** MessageBox（"当前已是最新版本"），
//   用户点「确定」后才 quit(w) 退出。所以"5 秒自动关闭"只适用于
//   autoClose 覆盖的失败/错误路径，**不适用于无更新路径**。
//
// 因此本用例的断言改为：
//   ① 有模态框出现（证明 KeyNoUpdateBox 提示确实给了）；
//   ② 点掉之后进程干净退出，退出码 0（证明 quit 链路没断）；
//   ③ version.json 逐字节不变（check 是只读命令）。
//
// 仍然能抓住最典型的回归：若 AutoCloseAfter/quit 被改成 Hide()，
// 点掉模态框后进程依然不退出 → ①之后的退出断言会失败。
// ────────────────────────────────────────────────────────────────
func TestCase42(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-ui-nonew"
	e.EnsureProject(t, proj, false, nil, nil)
	// 服务端停在 1.0.0（项目创建即 V1.0.0），客户端也是 1.0.0 → 无更新
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	uiExe := e.BuildUI(t)
	uiInClient := filepath.Join(c.Up(), "aly-client-ui.exe")
	CopyFile(t, uiExe, uiInClient)

	before := ReadText(filepath.Join(c.Up(), "version.json"))
	res := e.RunUIModeAuto(t, uiInClient, c, "check", proj, 30*time.Second, 40*time.Second)

	if res.ExitCode == -999 {
		t.Fatalf("UI 在 40s 内未退出（-999）：模态框已点掉=%v，"+
			"说明点掉「确定」之后 quit(w) 没有结束消息循环（AutoCloseAfter 可能被改成了 Hide()）",
			res.Dismissed)
	}
	True(t, res.Dismissed, "无更新时应弹出「当前已是最新版本」模态框（KeyNoUpdateBox）")
	Eq(t, "点掉提示后退出码", res.ExitCode, 0)
	Eq(t, "version.json 必须逐字节不变（check 是只读命令）",
		ReadText(filepath.Join(c.Up(), "version.json")), before)
	Eq(t, "不得产生任何版本快照", ListDirMatch(c.Pkg, "ApplicationFolder_*"), 0)
	t.Logf("UI check 无更新：点框=%v，退出码=%d，耗时 %s", res.Dismissed, res.ExitCode, res.Elapsed)
}

// ────────────────────────────────────────────────────────────────
// 用例 43 — UI `-mode check` 有更新：不崩溃、不越权
//
// 关键不变量：非强制更新时，**用户没点"更新"就绝不能下载**。
// 自动化不点按钮 → 断言"无快照、状态未变"。
// ────────────────────────────────────────────────────────────────
func TestCase43(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-ui-hasnew"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "readme.txt", "hello-1.1.0")
	// 非强制更新（不传 setForceUpdate）
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	uiExe := e.BuildUI(t)
	uiInClient := filepath.Join(c.Up(), "aly-client-ui.exe")
	CopyFile(t, uiExe, uiInClient)

	// 有更新 → 弹卡片等用户点「更新/稍后」。自动化不点按钮，
	// 所以进程常驻（-999）是**预期**；这里只确认它不崩、不越权。
	res := e.RunUIModeAuto(t, uiInClient, c, "check", "", 0, 40*time.Second)

	True(t, res.ExitCode == 0 || res.ExitCode == -999,
		"UI check 有更新时崩溃了，退出码=%d，输出：%s", res.ExitCode, res.Output)
	NotContains(t, res.Output, "panic:", "UI 不应 panic")

	// 核心不变量：没点「更新」→ 绝不下载、绝不改状态
	Eq(t, "未点\"更新\"时不得产生 1.1.0 快照",
		Exists(c.VerDir("1.1.0")), false)
	v := c.ReadVersion(t)
	Eq(t, "version_status 必须仍是 applied", v.VersionStatus, StatusApplied)
	Eq(t, "version 必须仍是 1.0.0", v.Version, "1.0.0")
	t.Logf("UI check 有更新：退出码=%d（-999=等用户点，属预期）", res.ExitCode)
}

// ────────────────────────────────────────────────────────────────
// 用例 44 — UI 进度刷新与单位显示
//
// 单位/千分比这类纯函数**必须走单测**，不在真实窗口里断言像素。
// 真实链路只验证"大文件能落地"。
// ────────────────────────────────────────────────────────────────
func TestCase44(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-ui-prog"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	BigFile(t, filepath.Join(e.DataDir(proj), "big.bin"), 30)
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	// ① 格式化与千分比：单测覆盖（1024 进制 + 单调不减）
	code, out := e.GoTestGopath(t, "aly/client/aly-client-ui/aly-client-winui/app",
		"-run", "TestFormatBytes|TestByteProgressPermille", "-v", "-count=1")
	Eq(t, "格式化单测退出码", code, 0)
	Contains(t, out, "--- PASS: TestFormatBytes", "app 单测缺 PASS")
	Contains(t, out, "--- PASS: TestByteProgressPermille", "app 单测缺 PASS")
	NotContains(t, out, "--- FAIL", "格式化单测不应有失败项")

	// ② 真实下载链路（走 CLI 等价路径，避开无人交互的窗口阻塞）
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, NoMain: true,
	})
	outs := e.Run(t, c, "download_update")
	last := outs[len(outs)-1]
	True(t, last.IsSuccess, "30MB 下载失败: %s", last.ErrorMsg)
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 失败")

	final := filepath.Join(c.Main(), "big.bin")
	Eq(t, "大文件落地大小", SizeOf(final), int64(30<<20))
	Eq(t, "MD5 与源一致", MD5Of(final), MD5Of(filepath.Join(e.DataDir(proj), "big.bin")))
}

// ────────────────────────────────────────────────────────────────
// 用例 45 — UI 下载中暂停/继续（.part 原子写 + ≤100MB 全量重下口径）
//
// 用 CLI 精确模拟"点暂停"：中断时 .part 必须保留（原子写的第一步），
// 重试必须能下完且 MD5 正确。
//
// ⚠ 口径修正（原注释写"续传必须接着写而不是重头下"，是错的）：
//   50MB < largeFileThreshold(100MB)，`DownloadFileWithResume` 的续传分支
//   （client/http_client.go:289 `if serverFileSize > largeFileThreshold`）
//   根本进不去，重试时是 `os.Create(partPath)` 从头写。这是**既有设计**
//   （小文件续传的三次握手不值当），真续传由用例 102（120MB）覆盖。
//   本用例因此只钉住"中断留 .part / 重试能下完 / 数据不错位"。
// ────────────────────────────────────────────────────────────────
func TestCase45(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-ui-pause"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	bigSrc := filepath.Join(e.DataDir(proj), "big.bin")
	BigFile(t, bigSrc, 50)
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, NoMain: true,
	})

	// ① 起下载，等 .part 出现（说明真的在写字节）
	part := filepath.Join(c.VerDir("1.1.0"), "big.bin.part")
	cmd, buf := e.RunAsync(c, "download_update")
	if !WaitFile(t, part, 60*time.Second) {
		Kill(t, cmd)
		t.Skipf("60s 内未出现 .part（本机太快或已直接落盘），异步输出：%s", buf.String())
	}
	// 再等一会让 .part 攒够字节再"暂停"
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && SizeOf(part) < 1<<20 {
		time.Sleep(50 * time.Millisecond)
	}
	Kill(t, cmd) // 等价点"暂停"：杀子进程，.part 保留
	sizeAtPause := SizeOf(part)
	True(t, sizeAtPause > 0, "暂停时 .part 应有部分字节，实际 %d", sizeAtPause)
	Eq(t, "暂停后 .part 必须保留（原子写语义：未 rename 前绝不直接写正式文件）", Exists(part), true)

	// ② 重试下完（≤100MB 走全量重下，语义正确性由 MD5 保证）
	outs := e.Run(t, c, "download_update")
	last := outs[len(outs)-1]
	True(t, last.IsSuccess, "重试下载失败: %s", last.ErrorMsg)

	done := filepath.Join(c.VerDir("1.1.0"), "big.bin")
	Eq(t, "最终大小", SizeOf(done), int64(50<<20))
	Eq(t, "MD5 与源一致（数据错位会在这里暴露）", MD5Of(done), MD5Of(bigSrc))
	Eq(t, ".part 应消失", Exists(part), false)
	t.Logf("暂停时 %d 字节，重试后 %d 字节", sizeAtPause, SizeOf(done))

	// ③ 断言"≤100MB 全量重下"这一**设计口径**，而不是假装在测续传。
	//
	// ⚠ 原实现是死断言，必须说明为什么改：
	//   旧代码写的是
	//       for _, o := range outs {
	//           if o.Progress.Status == "START" && o.Progress.DownloadedBytes > 0 {
	//               True(t, o.Progress.DownloadedBytes >= sizeAtPause/2, ...)
	//               break
	//           }
	//       }
	//   但 `DownloadFileWithResume`（client/http_client.go:289）的续传分支条件是
	//       if serverFileSize > largeFileThreshold   // 100MB，严格大于
	//   本用例用 50MB → 走全量重下分支：`os.Remove(partPath)` + `os.Create(partPath)`，
	//   从头写。而 START 行的 `downloaded_bytes` 传的是 `downloadedBytes`
	//   （download_update.go:275，**已完成文件的累计值**，不含当前文件）；第二轮
	//   app.exe 已校验通过、不在 downloadList 里，故 downloadList 只剩 big.bin，
	//   START 行 downloaded_bytes = 0 → 被 omitempty 省略 → 条件恒不成立。
	//   结果：这条断言**一次都没执行过**，用例 3/4 段通过、第③段静默跳过。
	//
	// 真续传由用例 102（120MB > 阈值）覆盖；这里改为钉住"≤100MB 不续传"，
	// 两道口径各自成立，任何一边被改都会有一条用例失败。
	start := progressStartFor(outs, "big.bin")
	True(t, start != nil,
		"必须存在 big.bin 的 START 行，否则下面的断言是空的。实际进度行: %v", progressFiles(outs))
	Eq(t, "≤100MB 不续传：big.bin 的 START 起点必须是 0（真续传见用例 102）",
		start.DownloadedBytes, 0)

	// ④ apply 后终态正确
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 失败")
	Eq(t, "应用后大小", SizeOf(filepath.Join(c.Main(), "big.bin")), int64(50<<20))
}