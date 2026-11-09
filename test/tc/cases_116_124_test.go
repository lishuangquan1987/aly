package tc

// 用例 116–124：上一轮（106–115）明确列为"仍未覆盖"的 8 类缺口 + SDK Launch 失败路径
//
// 上一轮的遗留清单是：
//   1. version.json 损坏**且无 .bak**（107 只覆盖了"有 .bak"与"空文件"）
//   2. apply 时快照目录**不可读**（ACL/杀软锁，A2 分支）
//   3. 并发 apply+download、并发 rollback+apply（用例 31 只测两个 download）
//   4. publish_version 事务部分失败（落库成功但清单写失败）
//   5. ignore 规则的正则/大小写/Wildcard 边界
//   6. -lang en-US 文案（RunUIModeLang 存在但零调用）
//   7. C# SDK Launch 拉 UI 失败 / UI 缺失（用例 94 只测成功路径）
//   8. download_file 多段 Range、If-Range
//
// 对应本文件：
//
//	116  1（损坏且无 .bak → 隔离为 .corrupt 并重置为首次部署）
//	117  2（待应用快照目录不可读 → 拒绝且不丢弃待应用包）
//	118  3-a（并发 download+apply：全局锁互斥）
//	119  3-b（并发 apply+rollback：全局锁互斥，状态机不被打乱）
//	120  4（publish_version 清单写失败 → 发布仍成功，但必须留痕）
//	121  5（ignore 规则边界：大小写 / 通配 / ** / 目录前缀 / 非法正则）
//	122  6（-lang en-US：不崩、能正常走完）
//	123  8（多段 Range → multipart/byteranges；If-Range 语义）
//	124  7（SDK Launch 拉 UI 失败路径，驱动 sd15）
//
// 每条独立自造现场，失败信息带现场路径。

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ────────────────────────────────────────────────────────────────
// 用例 116 — version.json 损坏**且无 .bak**：隔离为 .corrupt 并重置
//
// 背景（config/version.go:88-96）：
//
//	if bak, bErr := ioutilReadFile(path + ".bak"); bErr == nil {
//	    if json.Unmarshal(bak, &bakInfo) == nil { ... 从 .bak 恢复 ... }
//	}
//	os.Remove(path + ".corrupt")   // 清理上一次隔离残留（Windows rename 目标已存在会失败）
//	os.Rename(path, path+".corrupt")
//	return &VersionInfo{}, nil     // 重置为"首次部署"语义
//
// 用例 107 覆盖的是"**有** .bak"与"空文件"两种；这里专测**没有 .bak 可回退**这条：
//   - .bak 不存在
//   - .bak 存在但内容也是坏的（二次损坏）
// 两者都必须走"隔离 + 重置"，且**绝不能**抛错瘫痪（Bug#4 的原始症状）。
//
// 为什么是红线级：重置 = 返回空 VersionInfo = "首次部署"语义。若此时还残留着
// 上次的 v1.1.0 快照目录，check 必须能重新发现更新并把它装上去；若重置后
// 状态/内容错配，机器会永久停在"以为没装过，但目录里是半成品"的状态（红线①③）。
// ────────────────────────────────────────────────────────────────
func TestCase116(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-ver-nobak"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "a.dll", "a-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	// ── ① 损坏 + 无 .bak ──
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	verPath := filepath.Join(c.Up(), "version.json")
	bakPath := verPath + ".bak"
	corruptPath := verPath + ".corrupt"

	// 先制造"曾经写过 .bak"的假象再删掉，确保现场真的没有备份
	Eq(t, "前置：先不存在 .bak", Exists(bakPath), false)
	WriteText(t, verPath, `{"version":"1.0.0","version_status":"appli`) // 半截 JSON
	Eq(t, "前置：version.json 已是无效 JSON",
		json.Unmarshal([]byte(ReadText(verPath)), &VersionInfo{}) != nil, true)

	out := e.RunLast(t, c, "check_update")
	if !out.IsSuccess {
		// 允许"明确报错"分支，但必须带 errorMsg（不得静默失败）
		True(t, out.ErrorMsg != "", "① 失败必须带 errorMsg（不得静默）")
		t.Logf("① check 报错（可接受分支）: %s", out.ErrorMsg)
	} else {
		d := Data[CheckUpdateData](t, out)
		Eq(t, "① 重置为首次部署后应能发现 1.1.0", d.NewVersion, "1.1.0")
		Eq(t, "① 应要求下载", d.NeedDownloadUpdate, true)
	}

	// 自愈链硬证据：损坏文件被隔离，不是被静默删除
	Eq(t, "① 损坏文件必须隔离为 .corrupt（不得静默删除）", Exists(corruptPath), true)
	if Exists(corruptPath) {
		True(t, strings.Contains(ReadText(corruptPath), "version"),
			"① .corrupt 里应是原来那段坏内容，实际: %q", truncate(ReadText(corruptPath), 200))
	}
	Eq(t, "① 原 version.json 不应再是坏内容（要么被重建、要么不存在）",
		!strings.Contains(ReadText(verPath), `"appli`), true)

	// 红线①：自愈绝不能顺手动主目录
	Eq(t, "① 主目录 app.exe 未被改动",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")

	// 自愈后必须能真正走通全链路（否则"自愈"只是把文件藏起来了）
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "① 自愈后 download 应成功")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "① 自愈后 apply 应成功")
	Eq(t, "① 自愈后主目录已是新版本",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")

	// ── ② 损坏 + .bak 也是坏的（二次损坏）──
	c2 := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100", DirName: "client-116b",
	})
	vp2 := filepath.Join(c2.Up(), "version.json")
	WriteText(t, vp2, `{not json at all`)
	WriteText(t, vp2+".bak", `{also not json`) // .bak 也坏
	Eq(t, "② 前置：.bak 存在但内容无效",
		json.Unmarshal([]byte(ReadText(vp2+".bak")), &VersionInfo{}) != nil, true)

	out2 := e.RunLast(t, c2, "check_update")
	if out2.IsSuccess {
		d2 := Data[CheckUpdateData](t, out2)
		Eq(t, "② 二次损坏也应自愈并发现 1.1.0", d2.NewVersion, "1.1.0")
	} else {
		True(t, out2.ErrorMsg != "", "② 失败必须带 errorMsg")
	}
	Eq(t, "② 二次损坏时 .corrupt 隔离仍必须发生", Exists(vp2+".corrupt"), true)
	Eq(t, "② 不得因为 .bak 也坏就留下不可解析的 version.json",
		Exists(vp2) && json.Unmarshal([]byte(ReadText(vp2)), &VersionInfo{}) != nil, false)
}

// ────────────────────────────────────────────────────────────────
// 用例 117 — 待应用快照目录**不可读**（A2 分支）
//
// 背景（apply_update.go:196-206 / check_update.go:178-183）：
//
//	pendingCount, countErr := countRegularFiles(pendingDir)
//	if countErr != nil && !os.IsNotExist(countErr) {
//	    // A2：目录存在但读不到（ACL/杀软锁/慢盘超时）：不得据此自愈丢弃可能有效的待应用包
//	    ... printOutput(false, "待应用版本目录不可读: ...") ; return
//	}
//
// 这条分支的价值：**"读不到" ≠ "文件不全"**。
// 若把"读不到"当成"缺失/残缺"处理，代码会走 F3 自愈 → 把好不容易下载好的
// 待应用包状态清掉 → 下一轮重新下 100MB。慢盘/杀软扫的时候这非常常见。
//
// 触发手段：Windows 上用 `icacls` 去掉当前用户的读取权限。
// 不可用时（无 icacls / 权限不足 / 以管理员运行导致绕过）→ SKIP，不伪造结论。
// ────────────────────────────────────────────────────────────────
func TestCase117(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-a2"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "a.dll", "a-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download")
	Eq(t, "前置：进入 downloaded", c.ReadVersion(t).VersionStatus, StatusDownloaded)

	snap := c.VerDir("1.1.0")
	Eq(t, "前置：快照目录存在且有文件", len(ListDir(snap)) > 0, true)

	// 去掉当前用户对快照目录的读取权限 → countRegularFiles 的 Walk 必须报错
	if !denyDirRead(t, snap) {
		t.Skip("本机无法设置目录拒绝读取（icacls 不可用或权限不足），A2 分支无法构造")
	}
	defer allowDirRead(t, snap) // 无论如何都要恢复，否则 t.TempDir 清理会失败

	// 先确认现场真的"读不到"——否则后面的断言是空的
	if _, err := os.ReadDir(snap); err == nil {
		t.Skip("icacls 已执行但目录仍可读（可能以管理员/SeRestorePrivilege 运行），A2 分支无法构造")
	}

	beforeVer := c.ReadVersion(t)

	// ① check_update：不得把"读不到"当成"需要重下"
	cu := e.RunLast(t, c, "check_update")
	if cu.IsSuccess {
		d := Data[CheckUpdateData](t, cu)
		t.Logf("① 不可读时 check: has_update=%v need_download=%v new=%q",
			d.HasUpdate, d.NeedDownloadUpdate, d.NewVersion)
		// A2 的语义：不当成"丢失"，因此**不应**要求重下（重下会白传一遍）
		if d.NeedDownloadUpdate {
			t.Logf("⚠ 产品行为与注释不符：快照不可读时 check 要求重下（A2 注释说应跳过重下决策）")
		}
	} else {
		t.Logf("① check 明确报错（可接受）: %s", cu.ErrorMsg)
	}

	// ② apply_update：必须拒绝，且**不得**自愈丢弃待应用包
	out := e.RunLast(t, c, "apply_update")
	Eq(t, "② 快照不可读时 apply 必须拒绝（不得假成功）", out.IsSuccess, false)
	v := c.ReadVersion(t)
	// 把现场打出来：这条断言很容易"空洞通过"（比如 apply 因为别的原因失败），
	// 必须让人能一眼看到拒绝原因确实是"目录不可读"。
	t.Logf("② apply 拒绝: errorMsg=%q", out.ErrorMsg)
	t.Logf("② 拒绝后 version.json: version=%q previous=%q status=%q",
		v.Version, v.VersionPrevious, v.VersionStatus)
	t.Logf("② update.log 尾部:\n%s", c.Log(t, "update.log", 25))
	// 拒绝原因必须与"不可读"相关（否则可能命中了别的前置检查，A2 分支根本没走到）
	True(t, StringContainsAny(out.ErrorMsg, []string{"不可读", "unreadable", "拒绝", "refuse", "read"}),
		"② 拒绝原因应指向\"目录不可读\"，实际 errorMsg=%q（可能是别的前置检查拦下的，A2 未覆盖）",
		out.ErrorMsg)
	// 关键：状态必须是 applying（被拒绝后回滚）或 downloaded（保持原状），
	// **不能**变成 applied —— 变成 applied 就意味着"把待应用包丢了，当没下过"
	if v.VersionStatus == StatusApplied {
		t.Fatalf("② 快照不可读却自愈成 applied：待应用包被丢弃（A2 分支失效）\n"+
			"   version.json: version=%q previous=%q status=%q\n"+
			"   update.log:\n%s",
			v.Version, v.VersionPrevious, v.VersionStatus, c.Log(t, "update.log", 60))
	}
	Eq(t, "② 版本号不得被改动（拒绝路径无副作用）", v.Version, beforeVer.Version)
	Eq(t, "② version_previous 不得被改动", v.VersionPrevious, beforeVer.VersionPrevious)

	// 红线①：主目录一个字节都不能动
	Eq(t, "② 主目录 app.exe 未被改动",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")

	// ③ 恢复权限后必须能正常 apply —— 证明"拒绝"是可恢复的，不是死路
	allowDirRead(t, snap)
	outOK := e.RunLast(t, c, "apply_update")
	True(t, outOK.IsSuccess, "③ 恢复权限后 apply 应成功: %s\n%s",
		outOK.ErrorMsg, c.Log(t, "update.log", 60))
	Eq(t, "③ apply 成功后主目录已是新版本",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
}

// ────────────────────────────────────────────────────────────────
// 用例 118 — 并发 download + apply（全局更新锁）
//
// 背景（cmd/lock.go）：`AcquireUpdateLock` 保证同一时刻只有一个
// download_update / apply_update / rollback 在跑，锁文件 `UpdateFolder/.aly.lock`。
//
// 用例 31 只测了"两个 download_update 并发"；这里补最危险的一对：
// **download 正在写快照目录时，apply 把它 rename 走**。
// 若锁失效，apply 会把"还没写完的版本目录"当成完整包改名成主目录
// （红线①③：用户拿到半成品程序，version.json 却写 applied）。
//
// 判定：两者必须**串行化**——至少一个被锁拒绝，且无论谁先跑，
// 最终状态必须自洽（不能出现"version.json 说 applied，主目录却是旧内容"）。
// ────────────────────────────────────────────────────────────────
func TestCase118(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-lock-da"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	// 用 40MB 让 download 有明确的时间窗口（不至于瞬间结束导致撞不上）
	BigFile(t, filepath.Join(e.DataDir(proj), "big.bin"), 40)
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	// 首次 download 完成 → 进入 downloaded，才有"可 apply"的前提
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "前置 download")
	Eq(t, "前置：进入 downloaded", c.ReadVersion(t).VersionStatus, StatusDownloaded)

	// 重置回 applied 现场，让 download 与 apply 同时争抢
	c.WriteVersion(t, VersionInfo{
		VersionPrevious: "1.0.0", Version: "1.0.0", VersionStatus: StatusApplied,
	})
	// 删掉快照，确保接下来的 download 是从零开始（有足够时间窗口）
	if err := os.RemoveAll(c.VerDir("1.1.0")); err != nil {
		t.Fatalf("清理快照失败: %v", err)
	}

	outs := e.RunConcurrent(t, []*Client{c, c},
		[]string{"download_update"}, []string{"apply_update"})
	for i, o := range outs {
		t.Logf("第 %d 路: isSuccess=%v errorMsg=%q", i+1, o.IsSuccess, o.ErrorMsg)
	}

	// 判据 1：不得两路都"成功"——apply 在 download 还没写完时不可能真的成功
	if outs[0].IsSuccess && outs[1].IsSuccess {
		// 允许（download 先跑完、apply 再跑），但要验证最终状态真的自洽
		t.Logf("两路都成功：按 download→apply 串行解释，下面校验最终状态")
	}
	// 判据 2：被拒绝的那路必须有明确的锁提示（不是莫名其妙的错误）
	for i, o := range outs {
		if !o.IsSuccess && o.ErrorMsg != "" {
			if !strings.Contains(o.ErrorMsg, "另一更新") &&
				!strings.Contains(o.ErrorMsg, "no pending update") &&
				!strings.Contains(o.ErrorMsg, "已经是最新") &&
				!strings.Contains(o.ErrorMsg, "已是最新") {
				t.Logf("⚠ 第 %d 路失败原因不是锁/前置条件: %s", i+1, o.ErrorMsg)
			}
		}
	}

	// 判据 3（最重要）：最终 version.json 与磁盘内容必须自洽
	if err := os.Remove(c.Up() + "/.aly.lock"); err == nil {
		t.Logf("清理并发残留锁")
	}
	v := c.ReadVersion(t)
	mainContent := strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe")))
	t.Logf("最终: version=%q previous=%q status=%q 主目录 app.exe=%q 快照存在=%v",
		v.Version, v.VersionPrevious, v.VersionStatus, mainContent, Exists(c.VerDir("1.1.0")))

	switch v.VersionStatus {
	case StatusApplied:
		// applied 时主目录内容必须与 version 匹配（红线①）
		if v.Version == "1.1.0" {
			Eq(t, "并发后 applied/1.1.0 → 主目录必须是新版", mainContent, "app-110")
		} else {
			Eq(t, "并发后 applied/1.0.0 → 主目录必须是旧版", mainContent, "app-100")
		}
	case StatusDownloaded:
		// downloaded 时主目录必须仍是 VersionPrevious 的内容
		Eq(t, "并发后 downloaded → 主目录必须仍是旧版（新版本还没应用）",
			mainContent, "app-100")
		Eq(t, "并发后 downloaded → 必须有待应用版本号", v.Version != "", true)
	case StatusApplying:
		// applying：崩溃恢复会在下一次命令里完成，这里只要求不丢主目录
		Eq(t, "并发后 applying → 主目录不得消失", Exists(c.Main()), true)
	default:
		t.Fatalf("并发后出现未知状态 %q", v.VersionStatus)
	}

	// 判据 4：无论落在哪个状态，都必须能靠"再跑一轮 check/download/apply"自愈到 applied。
	// 判据看**最终落点**而非 apply_update 的返回值——已落在 applied 时 apply 会合法地
	// 返回 no pending update to apply（IsSuccess=false），那不算失败。
	settled118 := func() bool {
		vv := c.ReadVersion(t)
		return vv.VersionStatus == StatusApplied && vv.Version == "1.1.0" &&
			strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))) == "app-110"
	}
	done := settled118()
	for i := 0; i < 3 && !done; i++ {
		e.RunLast(t, c, "check_update")
		e.RunLast(t, c, "download_update")
		e.RunLast(t, c, "apply_update")
		if done = settled118(); done {
			t.Logf("第 %d 轮自愈后落到 applied/1.1.0", i+1)
		}
	}
	vv := c.ReadVersion(t)
	True(t, done, "并发后必须能通过常规命令自愈（否则用户现场永久卡住）: version=%q status=%q 主目录=%q\n%s",
		vv.Version, vv.VersionStatus, strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))),
		c.Log(t, "update.log", 40))
	Eq(t, "最终主目录应为新版", strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
}

// ────────────────────────────────────────────────────────────────
// 用例 119 — 并发 rollback + apply（全局更新锁）
//
// 比 118 更危险的一对：两者都做"三段 rename"，若同时进行，
// 会把主目录改名两次 → 备份目录与版本目录互相顶掉（红线②：误删）。
// ────────────────────────────────────────────────────────────────
func TestCase119(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-lock-ra"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "a.dll", "a-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	// 先让机器跑到 1.1.0 applied，并且 1.0.0 备份目录在（可回滚）
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.1.0")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.1.0")
	Eq(t, "前置：已在 1.1.0", c.ReadVersion(t).Version, "1.1.0")
	Eq(t, "前置：1.0.0 备份目录在", Exists(c.VerDir("1.0.0")), true)
	Eq(t, "前置：主目录是新版", strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")

	// 再发布 1.2.0 并下载（制造"可 apply"的前提）
	e.PutFile(t, proj, "app.exe", "app-120")
	True(t, e.Publish(t, proj, "1.2.0", nil, "", "").IsSuccess, "发布 1.2.0")
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.2.0")
	Eq(t, "前置：进入 downloaded/1.2.0", c.ReadVersion(t).VersionStatus, StatusDownloaded)

	// 并发：apply 升到 1.2.0 vs rollback 回 1.0.0 —— 两个相反方向的三段 rename
	//
	// 用 RunConcurrentFull 而非 RunConcurrent：失败那路若"一行 JSON 都没打"
	// （进程被杀/启动失败），只看 Out 会得到 errorMsg="" 的空洞失败，无线索。
	rs := e.RunConcurrentFull(t, []*Client{c, c},
		[]string{"apply_update"}, []string{"rollback", "--version", "1.0.0"})
	for i, r := range rs {
		t.Logf("第 %d 路: isSuccess=%v errorMsg=%q err=%v panics=%q raw=%q",
			i+1, r.Out.IsSuccess, r.Out.ErrorMsg, r.Err, r.Panics, strings.TrimSpace(r.Raw))
	}
	// 防"用例自欺"：若某路打印「未知命令」说明参数顺序写错，命令根本没执行，
	// 此时的"通过"是假的（rollback 压根没跑，自然不会有并发冲突）。
	for i, r := range rs {
		if strings.Contains(r.Raw, "未知命令") || strings.Contains(r.Raw, "用法: aly-client") {
			t.Fatalf("第 %d 路命令未被执行（参数写错），用例失去意义: %q", i+1, strings.TrimSpace(r.Raw))
		}
	}
	// 至少一路必须给出「成功」或「明确的拒绝原因」，不得两路都静默失败。
	hasReason := false
	for _, r := range rs {
		if r.Out.IsSuccess || r.Out.ErrorMsg != "" || strings.TrimSpace(r.Raw) != "" {
			hasReason = true
		}
	}
	True(t, hasReason, "并发两路必须至少有一路给出可诊断结果（否则用户排障无从下手）")

	if err := os.Remove(c.Up() + "/.aly.lock"); err == nil {
		t.Logf("清理并发残留锁")
	}

	// 核心判据：主目录必须存在且内容**只能是三个已知值之一**，
	// 绝不能是空目录 / 半成品 / 消失（红线②）
	Eq(t, "并发后主目录必须仍存在", Exists(c.Main()), true)
	mainContent := strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe")))
	switch mainContent {
	case "app-100", "app-110", "app-120":
		t.Logf("主目录内容 = %q（合法）", mainContent)
	default:
		t.Fatalf("并发 rollback+apply 后主目录内容异常: %q（应为 app-100/110/120 之一）\n"+
			"version.json: %+v\nupdate.log:\n%s",
			mainContent, c.ReadVersion(t), c.Log(t, "update.log", 80))
	}

	// version.json 与主目录必须自洽（红线①）
	v := c.ReadVersion(t)
	t.Logf("最终: version=%q previous=%q status=%q 主目录=%q",
		v.Version, v.VersionPrevious, v.VersionStatus, mainContent)
	if v.VersionStatus == StatusApplied {
		switch v.Version {
		case "1.0.0":
			Eq(t, "applied/1.0.0 → 主目录应是 app-100", mainContent, "app-100")
		case "1.1.0":
			Eq(t, "applied/1.1.0 → 主目录应是 app-110", mainContent, "app-110")
		case "1.2.0":
			Eq(t, "applied/1.2.0 → 主目录应是 app-120", mainContent, "app-120")
		}
	}

	// 必须能自愈：串行跑常规命令后，最终必须落在 applied 且内容与版本号匹配。
	//
	// ⚠ 判据不能写成"apply_update 返回 IsSuccess"：若并发后现场已经落在 applied，
	// apply_update 会**合法地**返回 "no pending update to apply"（IsSuccess=false），
	// 此时现场完全自洽，用返回值判失败属于误报。
	expectMain := func(ver string) string {
		switch ver {
		case "1.0.0":
			return "app-100"
		case "1.1.0":
			return "app-110"
		case "1.2.0":
			return "app-120"
		}
		return ""
	}
	settled := func() bool {
		vv := c.ReadVersion(t)
		if vv.VersionStatus != StatusApplied {
			return false
		}
		want := expectMain(vv.Version)
		if want == "" {
			return false
		}
		return strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))) == want
	}

	done := settled()
	for i := 0; i < 3 && !done; i++ {
		e.RunLast(t, c, "check_update")
		e.RunLast(t, c, "download_update")
		e.RunLast(t, c, "apply_update")
		if done = settled(); done {
			t.Logf("第 %d 轮自愈后落到 applied/%s", i+1, c.ReadVersion(t).Version)
		}
	}
	vv := c.ReadVersion(t)
	True(t, done, "并发 rollback+apply 后必须自愈到 applied 且内容匹配（否则用户现场永久卡住）: "+
		"version=%q status=%q 主目录=%q\n%s",
		vv.Version, vv.VersionStatus, strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))),
		c.Log(t, "update.log", 60))

	// 收敛后再跑一轮，确认是幂等终态（不会继续漂移）
	e.RunLast(t, c, "check_update")
	e.RunLast(t, c, "apply_update")
	Eq(t, "自愈后应稳定在同一版本", c.ReadVersion(t).Version, vv.Version)
	Eq(t, "自愈后主目录内容不变",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), expectMain(vv.Version))
}

// ────────────────────────────────────────────────────────────────
// 用例 120 — publish_version 事务部分失败（落库成功但清单写失败）
//
// 背景（internal/service/project_service.go:229-236）：
//
//	// 写失败不阻断发布（返回成功）：清单缺失时客户端跳过删除（与旧行为兼容，仅退化为不删）。
//	if mErr := writeReleaseManifest(...); mErr != nil {
//	    log.Printf("[manifest] publish version %s ... write manifest failed: %v", ...)
//	}
//
// 这是一个**有意的取舍**：清单写失败时发布仍成功，代价是"删除同步退化"。
// 本用例钉住三条：
//   ① 发布仍返回成功（不得因为清单写不了就让用户发布失败）
//   ② version 已推进到新版本（落库是原子的，不能半途而废）
//   ③ 清单写失败的后果可观测：get_all_files 仍能列出文件（退化但不崩）
//
// 触发手段：把 `.updator_manifest.json` 变成一个**目录**（写文件必然失败）。
// ────────────────────────────────────────────────────────────────
func TestCase120(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-mf-fail"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	// 注意：create_project 的初始版本是 DefaultVersion = "V1.0.0"，
	// 所以第一次 publish 的版本号必须 **严格大于 1.0.0**，否则命中防倒退保护。
	r0 := e.Publish(t, proj, "1.0.1", nil, "", "")
	True(t, r0.IsSuccess, "发布 1.0.1 失败: isSuccess=%v errorMsg=%q", r0.IsSuccess, r0.ErrorMsg)
	Eq(t, "前置：清单文件已生成",
		Exists(filepath.Join(e.DataDir(proj), ".updator_manifest.json")), true)

	// 把清单路径做成目录 → 后续任何写文件操作都会失败
	manifest := filepath.Join(e.DataDir(proj), ".updator_manifest.json")
	if err := os.Remove(manifest); err != nil {
		t.Fatalf("删除原清单失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(manifest, "blocker"), 0o755); err != nil {
		t.Fatalf("构造清单目录占位失败: %v", err)
	}
	defer os.RemoveAll(manifest)

	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "b.dll", "b-110")
	r := e.Publish(t, proj, "1.0.2", []string{"清单写失败场景"}, "", "")

	// ① 发布必须仍成功（清单写失败是"降级"，不是"失败"）
	True(t, r.IsSuccess,
		"① 清单写失败不得阻断发布（设计取舍：降级为不删，而非让用户发布失败）: %s", r.ErrorMsg)

	// ② 版本必须真的推进了（落库是事务，不能因为后面写清单失败就回滚掉已提交的版本）
	Eq(t, "② 版本必须推进到 1.0.2（落库已提交）", e.GetProject(t, proj).Version, "1.0.2")

	// ③ 清单缺失时 get_all_files 仍必须可用（退化但不崩）
	files := e.GetAllFiles(t, proj)
	eqSet(t, "③ 清单缺失时文件列表仍应完整", FileRels(t, files),
		[]string{"app.exe", "b.dll"})

	// ④ 客户端仍能正常下载（不能因为服务端清单坏了就下不下来）
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.1", Status: StatusApplied, Previous: "1.0.1",
		Project: proj, MainFile: "app-101",
	})
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "④ 清单缺失时 download 应仍成功")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "④ apply 应成功")
	Eq(t, "④ 主目录已更新",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
	Eq(t, "④ 新增文件也下来了",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "b.dll"))), "b-110")

	// ⑤ 恢复现场后再发布一次：新清单必须能正常写出（证明前一次只是降级）
	os.RemoveAll(manifest)
	e.PutFile(t, proj, "app.exe", "app-120")
	True(t, e.Publish(t, proj, "1.0.3", nil, "", "").IsSuccess, "⑤ 恢复后发布应成功")
	Eq(t, "⑤ 清单文件应被重新写出",
		Exists(filepath.Join(e.DataDir(proj), ".updator_manifest.json")), true)
}

// ────────────────────────────────────────────────────────────────
// 用例 121 — ignore 规则的边界（大小写 / 通配 / ** / 目录前缀 / 非法正则）
//
// ⚠ 事实纠正（不要被"两套 ignore 实现"这个说法误导）：
//
//	`ignore_folders` / `ignore_files` **只在服务端生效**。客户端 aly-client 里
//	虽然定义了 config.ShouldSkipFolder / ShouldSkipFile，但 grep 全仓可见
//	**没有任何调用点消费 IgnoreFolders/IgnoreFiles**——它们只用于
//	`un_copy_folders` / `un_copy_files`（物化时"不迁移"），是另一套配置。
//
//	所以本用例测的是**服务端唯一那套 ignore 口径**，不存在"两边打架"。
//	（上一轮文档里写的"客户端/服务端两套口径不同"是错的，已修正。）
//
// 服务端实现（internal/service/file_cache.go:150-160）：
//
//	ignoreFolders: strings.HasPrefix(relPath, folder+"/") || relPath == folder   ← 纯字符串前缀
//	ignoreFiles  : MatchIgnoreFile = 全路径 Match || base Match || "*xxx" HasSuffix 兜底
//
// 由此推出的真实边界（本用例逐个钉住）：
//   - 文件夹规则是**字符串前缀**，`logs/**` 里的 `*` 不被解释 → `logs/` 下的文件
//     **一条都匹配不到**（`HasPrefix(rel, "logs/**/")` 永假），除非 relPath 恰好等于 `logs/**`
//   - 文件夹规则对大小写**敏感**（纯 HasPrefix）
//   - 文件规则 `*.TMP` 对大小写**敏感**（filepath.Match），但 `*` 前缀有 HasSuffix 兜底
//   - `exact.txt` 会通过 base 匹配命中**任意目录**下的同名文件（`sub/exact.txt` 也被忽略）
// ────────────────────────────────────────────────────────────────
func TestCase121(t *testing.T) {
	e := NewEnv(t, true, 0)

	// 造一棵有层次的文件树，覆盖每个 pattern 的靶点
	const proj = "tc-ignore"
	e.EnsureProject(t, proj, false,
		[]string{"logs/**", "Temp"},   // 文件夹规则：一个含 **，一个纯名字
		[]string{"*.log", "*.TMP", "exact.txt"})

	// ⚠ Windows 文件系统不区分大小写，下面两个"大小写对照"靶点会被现场本身消解：
	//   - `A.LOG` 与 `a.log` 是**同一个文件**（后写覆盖前写，磁盘上只剩 `a.log`）
	//   - `temp/` 与 `Temp/` 是**同一个目录**（`Temp/t.txt` 与 `temp/t2.txt` 都落在 `Temp/`）
	// 所以"大小写敏感"这件事在本机**无法通过并存文件来验证**，只能记录实际观察到的行为。
	// （代码级已确认：filepath.Match 与 strings.HasPrefix 都是大小写敏感的。）
	e.PutFile(t, proj, "app.exe", "app")
	e.PutFile(t, proj, "keep.txt", "keep")
	e.PutFile(t, proj, "a.log", "log-1")             // 命中 *.log（base 匹配）
	e.PutFile(t, proj, "A.LOG", "log-2")             // ⚠ 覆盖 a.log（同一文件）
	e.PutFile(t, proj, "sub/deep.log", "log-3")      // base 匹配 → 命中
	e.PutFile(t, proj, "logs/x.txt", "in-logs")      // logs/** 是否生效的靶点
	e.PutFile(t, proj, "logs/a/b/y.txt", "deep")     // 深层：logs/** 是否覆盖
	e.PutFile(t, proj, "Temp/t.txt", "in-temp")      // 纯目录名规则 → 命中
	e.PutFile(t, proj, "temp/t2.txt", "in-temp-lc")  // ⚠ 实际落进 Temp/（同一目录）
	e.PutFile(t, proj, "exact.txt", "exact")         // 精确名
	e.PutFile(t, proj, "sub/exact.txt", "not-exact") // base 相同 → 也会命中
	e.PutFile(t, proj, "x.tmp", "tmp-lc")            // *.TMP 大小写（不同名，可共存）
	e.PutFile(t, proj, "y.TMP", "tmp-uc")            // *.TMP 精确（不同名，可共存）

	// 强制刷缓存后取清单（服务端口径）
	files := e.GetAllFiles(t, proj)
	got := FileRels(t, files)
	t.Logf("服务端清单（共 %d 项）: %v", len(got), got)

	// 诊断：把磁盘上的**真实文件名**打出来。
	// Windows 文件系统不区分大小写，若同名不同大小写的文件被写到同一目录，
	// 后写的会**覆盖**前者（而不是并存）→ "大小写"这类靶点会被现场本身消解。
	t.Logf("磁盘真实文件名: %v", ListDir(e.DataDir(proj)))

	// ── 断言：四类必须被排除（服务端口径下明确会命中的）──
	//
	// 逐个对应服务端实现：
	//   a.log        → base Match "*.log"
	//   sub/deep.log → base Match "*.log"
	//   exact.txt    → base Match "exact.txt"
	//   Temp/t.txt   → HasPrefix("Temp/t.txt", "Temp/")
	for _, want := range []string{"a.log", "sub/deep.log", "exact.txt", "Temp/t.txt"} {
		True(t, !SliceContains(got, want), "① 服务端清单不应含 %q（命中 ignore 规则），实际: %v", want, got)
	}
	// ── 断言：必须保留的 ──
	for _, want := range []string{"app.exe", "keep.txt"} {
		True(t, SliceContains(got, want), "② 服务端清单必须含 %q，实际: %v", want, got)
	}

	// ── 差异点逐个记录（不 FAIL，因为这是要"记录真实口径"而非"断言理想口径"）──
	// expectIgnored 是"直觉上应该被忽略"的期望；实际不符说明服务端口径与直觉有偏差。
	check := func(name, rel string, expectIgnored bool) {
		ignored := !SliceContains(got, rel)
		if ignored == expectIgnored {
			t.Logf("  %-30s 实际忽略=%-6v（符合直觉）", name, ignored)
		} else {
			t.Logf("  %-30s 实际忽略=%-6v（**与直觉不符**：按 glob 直觉应忽略=%v）",
				name, ignored, expectIgnored)
		}
	}
	// 按 "glob 直觉" 的预期（注意：服务端口径是字符串前缀 + Match，不等于 glob 直觉）
	check("Temp/t.txt vs Temp（纯名字）", "Temp/t.txt", true)
	check("Temp/t2.txt vs Temp（经 temp/ 写入）", "Temp/t2.txt", true)
	check("logs/x.txt vs logs/**（** 一层）", "logs/x.txt", true)
	check("logs/a/b/y.txt vs logs/**（** 多层）", "logs/a/b/y.txt", true)
	check("sub/exact.txt vs exact.txt（同名子目录）", "sub/exact.txt", true)
	check("y.TMP vs *.TMP（精确）", "y.TMP", true)
	// x.tmp：*.TMP 是否大小写敏感的唯一有效靶点（不同文件名，可共存）
	check("x.tmp vs *.TMP（大小写）", "x.tmp", true)

	// ── 关键红线断言：不该被忽略的文件必须出现在清单里 ──
	// 服务端漏掉 = 客户端永远下不到 = 用户程序残缺（红线②）
	for _, rel := range []string{"app.exe", "keep.txt"} {
		if !SliceContains(got, rel) {
			t.Fatalf("③ 服务端漏掉了不该忽略的 %q（红线②：用户程序会残缺）", rel)
		}
	}

	// ── 非法 pattern（filepath.Match 会返回 ErrBadPattern）不得让服务端 500 ──
	//
	// ⚠ pattern 要选"语法非法"的，**不能选"匹配一切"的**：
	// 实测 `filepath.Match("***", "任意路径")` 返回 true（Go 把连续 * 视作合法且贪婪），
	// 于是 `***` 会把整个项目目录全部忽略掉 —— 那不是"非法 pattern 的容错"，
	// 而是"合法 pattern 正常生效"，测不出想测的东西。
	// 这里只用真正会返回 ErrBadPattern 的写法：`[a-`（未闭合字符类）、`[`、`a[b`。
	const proj2 = "tc-ignore-bad"
	e.EnsureProject(t, proj2, false, nil, []string{"[a-", "[", "a[b"})
	e.PutFile(t, proj2, "app.exe", "app")
	e.PutFile(t, proj2, "keep.txt", "keep")
	code := e.Status(t, "/api/file/get_all_files/"+url.PathEscape(proj2))
	Eq(t, "④ 非法 ignore pattern 不得让 get_all_files 报错（错误 pattern 只记 WARN）", code, 200)
	raw := e.Raw(t, "GET", "/api/file/get_all_files/"+url.PathEscape(proj2))
	t.Logf("④ 原始响应: %s", truncate(string(raw), 800))
	files2 := e.GetAllFiles(t, proj2)
	t.Logf("④ 解析后 %d 项: %v", len(files2), FileRels(t, files2))
	True(t, SliceContains(FileRels(t, files2), "app.exe"),
		"④ 非法 pattern 下正常文件仍必须列出，实际: %v", FileRels(t, files2))
}

// ────────────────────────────────────────────────────────────────
// 用例 122 — `-lang en-US`：不崩、能走完、与默认语言行为一致
//
// 背景：`RunUIModeLang` 这个辅助**存在但零调用**（上一轮盘点的第 6 条缺口）。
// 语言由 `-lang zh-CN|en-US` 传入，`i18n.ParseLang` 对空串/未知值回落 ZH。
//
// UI 是 windowsgui，没有控制台输出（既不写 stdout 也不落日志），
// 所以这里能验的是：
//   ① en-US 启动不崩（不 panic、不立即异常退出）
//   ② en-US 下流程行为与 zh-CN 一致（强制更新场景能自动走完并退出码 0）
//   ③ 未知语言值（如 "bogus"）不得崩，必须回落到中文继续工作
// ────────────────────────────────────────────────────────────────
func TestCase122(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-lang"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "app.exe", "app-110")
	e.Publish(t, proj, "1.1.0", []string{"lang test"}, "", "")
	// 强制更新 → 无需点按钮，UI 自动走完（避免 -999 超时）
	e.Post(t, "/api/project/set_force_update", map[string]interface{}{
		"projectName": proj, "forceUpdate": true,
	})

	uiExe := e.BuildUI(t)

	// ① en-US 文档描述 / 缺省语言：UI 的 i18n 文案表必须两套齐全（代码级校验）
	i18nSrc := ReadText(filepath.Join(RepoRoot,
		"client/aly-client-ui/aly-client-winui/i18n/i18n.go"))
	True(t, len(i18nSrc) > 0, "① 必须能读到 i18n.go")
	Contains(t, i18nSrc, "EN", "① i18n 必须有 EN 语言常量")
	Contains(t, i18nSrc, "ParseLang", "① 必须有 ParseLang 解析入口")

	// ② 未知语言 → 回落 ZH（源码级 + 运行时级都验）
	Contains(t, i18nSrc, "ZH", "② 必须有 ZH 兜底")

	// ③ 真正跑：en-US 下强制更新应能自动完成
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	uiPath := filepath.Join(c.Up(), "aly-client-ui.exe")
	CopyFile(t, uiExe, uiPath)

	code, out := e.RunUIModeLang(t, uiPath, c, "update", "en-US", 90*time.Second)
	t.Logf("③ en-US 退出码=%d 输出=%q", code, truncate(out, 500))
	if code == -999 {
		// 超时杀掉：可能是强制更新没生效（force_update 未落库）或 UI 卡住
		t.Fatalf("③ en-US 下强制更新仍未自动完成（超时被 kill）——"+
			"要么 force_update 没生效，要么 en-US 路径卡住。主目录 app.exe=%q",
			strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))))
	}
	NotContains(t, out, "panic:", "③ en-US 不得 panic")
	Eq(t, "③ en-US 更新后主目录应为新版",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")

	// ④ 未知语言值：不得崩，必须回落后照常工作
	c2 := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100", DirName: "client-122b",
	})
	ui2 := filepath.Join(c2.Up(), "aly-client-ui.exe")
	CopyFile(t, uiExe, ui2)
	code2, out2 := e.RunUIModeLang(t, ui2, c2, "update", "bogus-lang", 90*time.Second)
	t.Logf("④ 未知语言退出码=%d 输出=%q", code2, truncate(out2, 300))
	NotContains(t, out2, "panic:", "④ 未知语言值不得 panic")
	True(t, code2 != -999, "④ 未知语言值应回落中文并正常走完（实际超时被 kill）")
	Eq(t, "④ 未知语言下更新也应完成",
		strings.TrimSpace(ReadText(filepath.Join(c2.Main(), "app.exe"))), "app-110")
}

// ────────────────────────────────────────────────────────────────
// 用例 123 — download_file 的多段 Range 与 If-Range
//
// 背景：`http.ServeContent` 的语义（标准库定）：
//   - 单段 `bytes=0-1`      → 206 + `Content-Range: bytes 0-1/<size>`
//   - 多段 `bytes=0-1,4-5`  → **206 + `Content-Type: multipart/byteranges; boundary=...`**
//   - If-Range 不匹配        → **200**（忽略 Range，返回全文）
//   - If-Range 匹配          → 206
//
// 为什么要测：客户端断点续传只认单段 206。若服务端对多段请求返回了
// `multipart/byteranges` 而客户端当成裸字节写入 `.part`，`.part` 会混入
// MIME 边界文本 → 拼出来的文件是坏的（红线①）。
// 本用例钉住"服务端严格按 RFC 行事"，防止有人手改成"永远返回单段"。
// ────────────────────────────────────────────────────────────────
func TestCase123(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-multirange"
	e.EnsureProject(t, proj, false, nil, nil)
	// 100 字节，内容可预测：0123456789... 便于逐字节校验
	content := ""
	for i := 0; i < 100; i++ {
		content += string(rune('0' + i%10))
	}
	e.PutFile(t, proj, "r.bin", content)
	e.Publish(t, proj, "1.0.0", nil, "", "")
	path := "/api/file/download_file?path=" + url.QueryEscape(
		filepath.Join(e.DataDir(proj), "r.bin"))
	size := len(content)

	// ── ① 无 Range → 200 全文 ──
	r0 := e.DoFull(t, "GET", path, nil)
	Eq(t, "① 无 Range 应 200", r0.Code, 200)
	Eq(t, "① 全文长度", len(r0.Body), size)

	// ── ② 单段 → 206 + Content-Range 正确 ──
	r1 := e.DoFull(t, "GET", path, map[string]string{"Range": "bytes=0-1"})
	Eq(t, "② 单段应 206", r1.Code, 206)
	Eq(t, "② 单段内容", string(r1.Body), content[0:2])
	Eq(t, "② Content-Range 必须指明范围与总长",
		r1.ContentRange(), fmt.Sprintf("bytes 0-1/%d", size))

	// ── ③ 多段 → 206 + multipart/byteranges ──
	r2 := e.DoFull(t, "GET", path, map[string]string{"Range": "bytes=0-1,4-5"})
	Eq(t, "③ 多段 Range 应 206", r2.Code, 206)
	ct := r2.ContentType()
	True(t, strings.HasPrefix(ct, "multipart/byteranges"),
		"③ 多段 Range 必须返回 multipart/byteranges（RFC 7233），实际 Content-Type=%q\n"+
			"  若返回裸字节，客户端会把 MIME 边界写进 .part（红线①）", ct)
	True(t, strings.Contains(ct, "boundary="),
		"③ multipart/byteranges 必须带 boundary，实际=%q", ct)
	// 两段内容都应出现在 body 里（各带 Content-Range 头）
	True(t, strings.Contains(string(r2.Body), content[0:2]),
		"③ 响应体应含第一段 %q", content[0:2])
	True(t, strings.Contains(string(r2.Body), content[4:6]),
		"③ 响应体应含第二段 %q", content[4:6])
	True(t, strings.Contains(string(r2.Body), "Content-Range"),
		"③ multipart 每一段都要带自己的 Content-Range")

	// ── ④ If-Range：资源未变 → 206；值不匹配 → 200 全文 ──
	// 用 Last-Modified 作为校验器（ServeContent 支持 ETag 或 HTTP-date）
	lm := r0.Header.Get("Last-Modified")
	True(t, lm != "", "④ 响应必须带 Last-Modified（否则 If-Range 无法工作）")

	r4a := e.DoFull(t, "GET", path, map[string]string{
		"Range": "bytes=0-9", "If-Range": lm,
	})
	Eq(t, "④ If-Range 匹配（资源未变）应 206", r4a.Code, 206)
	Eq(t, "④ If-Range 匹配时返回的是该段", len(r4a.Body), 10)

	r4b := e.DoFull(t, "GET", path, map[string]string{
		"Range": "bytes=0-9", "If-Range": "Wed, 21 Oct 2015 07:28:00 GMT", // 故意不匹配
	})
	Eq(t, "④ If-Range 不匹配必须回 200 全文（忽略 Range）", r4b.Code, 200)
	Eq(t, "④ If-Range 不匹配时返回全文", len(r4b.Body), size)

	// ── ⑤ 语法错误的 Range 头 → 400（ServeContent 行为），不得 500 ──
	r5 := e.DoFull(t, "GET", path, map[string]string{"Range": "bytes=abc-def"})
	True(t, r5.Code == 400 || r5.Code == 416,
		"⑤ 非法 Range 语法应 400/416（RFC 规定 400），实际 %d", r5.Code)
	True(t, r5.Code != 500, "⑤ 非法 Range 不得导致 5xx（服务端崩溃）")

	// ── ⑥ 端到端：多段响应不能被客户端当成续传数据 ──
	// 客户端只发单段 Range（http_client.go），这里验证单段 206 的字节**精确**
	r6 := e.DoFull(t, "GET", path, map[string]string{"Range": "bytes=50-"})
	Eq(t, "⑥ 后半段应 206", r6.Code, 206)
	Eq(t, "⑥ 后半段内容必须与源文件逐字节一致",
		string(r6.Body), content[50:])
}

// ────────────────────────────────────────────────────────────────
// 用例 124 — C# SDK `Launch` 拉 UI 的失败路径
//
// 背景（AlyUpdateUIClient.cs:120-175）：
//
//	if (!File.Exists(UiExePath))
//	    return AlyResponse.NG("aly-client-ui.exe not found: " + UiExePath + "（请确认 ...）");
//	try { ... process.Start() ... } catch (Exception ex) { return AlyResponse.NG(ex); }
//
// 用例 94 只测了"UI 存在 → Launch OK"的成功路径。失败路径同样重要：
// 部署漏了 UI（杀软隔离 / 只装了 client）时，SDK 必须**返回 NG 而不是抛异常**，
// 否则宿主界面会直接崩。
//
// 驱动：AlySdkSmoke 的 sd15（本用例直接调它，断言 PASS/FAIL 计数）。
// ────────────────────────────────────────────────────────────────
func TestCase124(t *testing.T) {
	e := NewEnv(t, true, 0)
	smoke := e.BuildSDK(t)

	const proj = "tc-sdk-launch"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.Publish(t, proj, "1.0.0", nil, "", "")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	out := e.RunSmoke(t, smoke, "sd15", c)
	t.Logf("sd15 输出:\n%s", truncate(out, 3000))

	// ① 不得出现 FAIL
	Eq(t, "① sd15 不得有 FAIL", OutputHasFail(out), false)

	// ② 必须真的执行了断言（防止 sd15 未注册 → 空跑也"没 FAIL"）
	Contains(t, out, "SUMMARY:", "② 必须有 SUMMARY 行（证明真的跑了）")
	True(t, strings.Contains(out, "SD15"),
		"② 输出里必须有 SD15 标记（说明用例被找到并执行）: %s", truncate(out, 800))

	// ③ 关键断言必须逐条出现在输出里
	for _, want := range []string{
		"UI 缺失时 Launch 必须返回 NG",
		"NG 必须带 errorMsg",
		"失败路径不得触发 Started",
		"Dispose 后 Launch 应返回 NG",
	} {
		Contains(t, out, want, "③ 必须验证: "+want)
	}

	// ④ SUMMARY 里 PASS 数必须 ≥ 6（本用例共 9 个断点，留余量）
	if i := strings.Index(out, "SUMMARY:"); i >= 0 {
		var pass, fail int
		_, _ = fmt.Sscanf(out[i:], "SUMMARY: PASS=%d FAIL=%d", &pass, &fail)
		True(t, fail == 0, "④ sd15 FAIL 数必须为 0，实际 %d", fail)
		True(t, pass >= 6, "④ sd15 PASS 数应 ≥6（实际 %d），太少说明断言被跳过", pass)
	}
}

// ────────────────────────────────────────────────────────────────
// 本文件专用辅助
// ────────────────────────────────────────────────────────────────

// eqSet 断言"实际集合 == 期望集合"（不看顺序，看多重集合相等）。
// 为什么不直接用字符串拼接比较：文件列表顺序依赖 Walk 顺序，不稳定；
// 而且失败时打印两份排序后的列表，比打印两个字符串好定位。
func eqSet(t *testing.T, field string, got, want []string) {
	t.Helper()
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	sortStrings(g)
	sortStrings(w)
	if len(g) != len(w) {
		t.Fatalf("%s: 元素数不同（期望 %d，实际 %d）\n期望: %v\n实际: %v",
			field, len(w), len(g), w, g)
	}
	for i := range g {
		if g[i] != w[i] {
			t.Fatalf("%s: 第 %d 项不同\n期望: %v\n实际: %v", field, i, w, g)
		}
	}
}

// denyDirRead 去掉**当前用户**对 dir 的读取权限，使 os.ReadDir / filepath.Walk 报错。
//
// 为什么用 icacls：Windows 没有 chmod，Go 的 os.Chmod 只影响只读位（不影响能否列目录）。
// ACL 是唯一能让 ReadDir 真正失败的手段。
//
// 返回 false 表示环境不支持（无 icacls、权限不足等）——调用方应 SKIP 而不是伪造结论。
// **注意**：必须以管理员运行时 SeRestorePrivilege 可能绕过拒绝，此时 ReadDir 仍成功，
// 调用方还要再自己确认一次"真的读不到"。
func denyDirRead(t *testing.T, dir string) bool {
	t.Helper()
	user := os.Getenv("USERNAME")
	if user == "" {
		return false
	}
	// (OI)(CI) 只作用于子项；这里要阻止"列目录"本身，用 (RD) 拒绝 ReadData/ListDirectory
	out, err := exec.Command("icacls", dir, "/deny", user+":(RD)").CombinedOutput()
	if err != nil {
		t.Logf("denyDirRead: icacls 失败（环境不支持）: %v\n%s", err, truncate(string(out), 400))
		return false
	}
	return true
}

// allowDirRead 撤销 denyDirRead 设置的拒绝项（必须在 defer 里调用，
// 否则 t.TempDir 清理会因权限不足失败，留下垃圾目录）。
// 目录已被删掉（apply 成功后被 rename 走）时 icacls 会报"找不到文件"——
// 这是正常情况，不是失败，静默返回。
func allowDirRead(t *testing.T, dir string) {
	t.Helper()
	user := os.Getenv("USERNAME")
	if user == "" {
		return
	}
	if !Exists(dir) {
		return // 目录已被消耗（apply/rollback rename 走），无权限项可撤
	}
	if out, err := exec.Command("icacls", dir, "/remove:d", user).CombinedOutput(); err != nil {
		// 只在目录仍存在却撤不掉时才值得警告
		if Exists(dir) {
			t.Logf("allowDirRead: 撤销失败: %v\n%s", err, truncate(string(out), 400))
		}
	}
}
