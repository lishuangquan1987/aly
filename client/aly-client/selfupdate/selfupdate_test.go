package selfupdate

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mkTmp 创建隔离临时目录（Go 1.10 兼容：不用 t.TempDir）。
func mkTmp(t *testing.T) (string, func()) {
	t.Helper()
	dir, err := ioutil.TempDir("", "selfupdate-ut")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	return dir, func() { os.RemoveAll(dir) }
}

func writeF(t *testing.T, path, content string) {
	t.Helper()
	if err := ioutil.WriteFile(path, []byte(content), 0755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readF(t *testing.T, path string) string {
	t.Helper()
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// ────────────────────────────────────────────────────────────────
// SHA256Of / SameFile / CopyAndVerify
// ────────────────────────────────────────────────────────────────

func TestSHA256OfAndSameFile(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	c := filepath.Join(dir, "c")
	writeF(t, a, "hello")
	writeF(t, b, "hello")
	writeF(t, c, "world")

	same, err := SameFile(a, b)
	if err != nil || !same {
		t.Fatalf("SameFile(a,b) = %v, %v; want true, nil", same, err)
	}
	same, err = SameFile(a, c)
	if err != nil || same {
		t.Fatalf("SameFile(a,c) = %v, %v; want false, nil", same, err)
	}
	// 缺文件必须报错（调用方决定跳过策略，本包不吞）
	if _, err = SameFile(a, filepath.Join(dir, "missing")); err == nil {
		t.Fatal("SameFile with missing file should error")
	}
}

func TestCopyAndVerifyKeepsMode(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	src := filepath.Join(dir, "src")
	writeF(t, src, "payload")
	tmp := filepath.Join(dir, "dst.new")

	if err := CopyAndVerify(src, tmp); err != nil {
		t.Fatalf("CopyAndVerify: %v", err)
	}
	if got := readF(t, tmp); got != "payload" {
		t.Fatalf("tmp content = %q", got)
	}
	// Unix 下可执行位必须保留（Windows 下 Chmod 仅最小有效位，无副作用）
	fi, err := os.Stat(tmp)
	if err != nil {
		t.Fatalf("stat tmp: %v", err)
	}
	if fi.Mode()&0100 == 0 {
		t.Fatalf("exec bit lost: %v", fi.Mode())
	}
}

// ────────────────────────────────────────────────────────────────
// ReplaceFromFile：成功 / 重试后成功 / 最终失败保留旧目标
// ────────────────────────────────────────────────────────────────

func TestReplaceFromFileSuccess(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	writeF(t, src, "V2")
	writeF(t, dst, "V1")

	if err := ReplaceFromFile(src, dst, Options{Retries: 1, Delay: time.Millisecond}); err != nil {
		t.Fatalf("ReplaceFromFile: %v", err)
	}
	if got := readF(t, dst); got != "V2" {
		t.Fatalf("dst = %q; want V2", got)
	}
	// 成功后不应残留 .new
	if _, err := os.Stat(dst + ".new"); !os.IsNotExist(err) {
		t.Fatalf(".new should not remain after success (err=%v)", err)
	}
}

func TestReplaceFromFileRetryThenSuccess(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	writeF(t, src, "V2")
	writeF(t, dst, "V1")

	calls := 0
	failFirst := 2 // 前两次 Move 失败，第三次成功
	opts := Options{
		Retries: 3,
		Delay:   time.Millisecond,
		Move: func(s, d string) error {
			calls++
			if calls <= failFirst {
				return os.ErrPermission
			}
			return os.Rename(s, d)
		},
	}
	if err := ReplaceFromFile(src, dst, opts); err != nil {
		t.Fatalf("ReplaceFromFile with retry: %v", err)
	}
	if calls != 3 {
		t.Fatalf("move calls = %d; want 3", calls)
	}
	if got := readF(t, dst); got != "V2" {
		t.Fatalf("dst = %q; want V2", got)
	}
}

func TestReplaceFromFileFailureKeepsDst(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	writeF(t, src, "V2")
	writeF(t, dst, "V1")

	// Move 永远失败：重试耗尽后报错，且旧 dst 必须原样保留（原子性契约）
	opts := Options{
		Retries: 2,
		Delay:   time.Millisecond,
		Move:    func(s, d string) error { return os.ErrPermission },
	}
	err := ReplaceFromFile(src, dst, opts)
	if err == nil {
		t.Fatal("expected error when move always fails")
	}
	if got := readF(t, dst); got != "V1" {
		t.Fatalf("dst must keep old content on failure, got %q", got)
	}
	// 失败后 .new 应被清理（不残留半截文件）
	if _, statErr := os.Stat(dst + ".new"); !os.IsNotExist(statErr) {
		t.Fatalf(".new should be cleaned after final failure (err=%v)", statErr)
	}
}

// ────────────────────────────────────────────────────────────────
// ReplaceRunningSelf：成功 / 失败回滚
// ────────────────────────────────────────────────────────────────

func TestReplaceRunningSelfSuccess(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	self := filepath.Join(dir, "aly-client.exe")
	src := filepath.Join(dir, "new-from-main")
	writeF(t, self, "V1")
	writeF(t, src, "V2")

	oldPath, err := ReplaceRunningSelf(self, src, Options{Retries: 1, Delay: time.Millisecond})
	if err != nil {
		t.Fatalf("ReplaceRunningSelf: %v", err)
	}
	if got := readF(t, self); got != "V2" {
		t.Fatalf("self = %q; want V2", got)
	}
	// .old 是被替换下来的旧版，且内容可核对
	if got := readF(t, oldPath); got != "V1" {
		t.Fatalf("old backup = %q; want V1", got)
	}
	if filepath.Base(oldPath)[:len("aly-client.exe.old")] != "aly-client.exe.old" {
		t.Fatalf("old path naming unexpected: %s", oldPath)
	}
}

func TestReplaceRunningSelfRollbackOnMoveFailure(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	self := filepath.Join(dir, "aly-client.exe")
	src := filepath.Join(dir, "new-from-main")
	writeF(t, self, "V1")
	writeF(t, src, "V2")

	// 落位永远失败：必须回滚到 V1，且 self 必须存在（宁可失败不能消失）
	opts := Options{
		Retries: 1,
		Delay:   time.Millisecond,
		Move:    func(s, d string) error { return os.ErrPermission },
	}
	_, err := ReplaceRunningSelf(self, src, opts)
	if err == nil {
		t.Fatal("expected error when move always fails")
	}
	if got := readF(t, self); got != "V1" {
		t.Fatalf("self must be rolled back to V1, got %q", got)
	}
}

func TestReplaceRunningSelfOldNameConflict(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	self := filepath.Join(dir, "app.exe")
	src := filepath.Join(dir, "new")
	writeF(t, self, "V1")
	writeF(t, src, "V2")
	// 预先占用 aly-client.exe.old 与 .old.1，验证侧路名避让
	writeF(t, filepath.Join(dir, "app.exe.old"), "old-junk")
	writeF(t, filepath.Join(dir, "app.exe.old.1"), "old-junk-1")

	oldPath, err := ReplaceRunningSelf(self, src, Options{Retries: 1, Delay: time.Millisecond})
	if err != nil {
		t.Fatalf("ReplaceRunningSelf: %v", err)
	}
	// 占用避让：不应覆盖已存在的 .old / .old.1
	if oldPath != filepath.Join(dir, "app.exe.old.2") {
		t.Fatalf("aside path = %s; want app.exe.old.2", oldPath)
	}
	if got := readF(t, filepath.Join(dir, "app.exe.old")); got != "old-junk" {
		t.Fatalf("pre-existing .old was overwritten: %q", got)
	}
}

// ────────────────────────────────────────────────────────────────
// CleanArtifacts / NextAsidePath
// ────────────────────────────────────────────────────────────────

func TestCleanArtifacts(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	dst := filepath.Join(dir, "app.exe")
	writeF(t, dst, "current")
	writeF(t, dst+".new", "half-written")
	writeF(t, dst+".old", "previous")
	writeF(t, dst+".old.1", "previous-1")
	// 无关文件不得误删
	writeF(t, dst+".other", "keep-me")

	CleanArtifacts(dst)
	for _, gone := range []string{dst + ".new", dst + ".old", dst + ".old.1"} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed", gone)
		}
	}
	if _, err := os.Stat(dst + ".other"); err != nil {
		t.Fatalf("unrelated file must survive: %v", err)
	}
	if got := readF(t, dst); got != "current" {
		t.Fatalf("dst must survive cleanup, got %q", got)
	}
}

func TestCleanArtifactsKeepsOldWhenDstMissing(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	// dst 缺失（上次替换失败且回滚也失败的极端现场）：.old 是最后备份，不能清
	writeF(t, filepath.Join(dir, "app.exe.old"), "last-resort")
	writeF(t, filepath.Join(dir, "app.exe.new"), "junk")

	CleanArtifacts(filepath.Join(dir, "app.exe"))
	if got := readF(t, filepath.Join(dir, "app.exe.old")); got != "last-resort" {
		t.Fatalf(".old must survive when dst is missing, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "app.exe.new")); !os.IsNotExist(err) {
		t.Fatalf(".new should still be removed (err=%v)", err)
	}
}

func TestNextAsidePath(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	dst := filepath.Join(dir, "app.exe")
	if got := NextAsidePath(dst, ".old"); got != dst+".old" {
		t.Fatalf("first aside = %s", got)
	}
	writeF(t, dst+".old", "x")
	if got := NextAsidePath(dst, ".old"); got != dst+".old.1" {
		t.Fatalf("second aside = %s", got)
	}
	writeF(t, dst+".old.1", "y")
	if got := NextAsidePath(dst, ".old"); got != dst+".old.2" {
		t.Fatalf("third aside = %s", got)
	}
}

// ────────────────────────────────────────────────────────────────
// Journal：计数 / 跳过 / 窗口重置 / 落盘往返
// ────────────────────────────────────────────────────────────────

func TestJournalShouldSkipAndWindowReset(t *testing.T) {
	var j Journal
	if j.ShouldSkip(3, 30*time.Minute) {
		t.Fatal("empty journal should not skip")
	}

	j.MarkFailure("boom-1")
	j.MarkFailure("boom-2")
	if j.Count != 2 {
		t.Fatalf("count = %d; want 2", j.Count)
	}
	if j.ShouldSkip(3, 30*time.Minute) {
		t.Fatal("2 failures < max 3, should not skip")
	}
	j.MarkFailure("boom-3")
	if !j.ShouldSkip(3, 30*time.Minute) {
		t.Fatal("3 consecutive recent failures should skip")
	}

	// 最近失败已在窗口外：允许重试（给瞬时故障恢复的机会）
	j.LastTS = time.Now().Unix() - int64(31*time.Minute/time.Second)
	if j.ShouldSkip(3, 30*time.Minute) {
		t.Fatal("failures older than window should not skip")
	}

	// 窗口外再失败一次：计数应重置为 1，而不是累加到 4
	j.MarkFailure("boom-after-window")
	if j.Count != 1 {
		t.Fatalf("count after window = %d; want 1", j.Count)
	}
}

func TestJournalMarkSuccess(t *testing.T) {
	var j Journal
	j.MarkFailure("boom")
	j.MarkFailure("boom")
	j.MarkSuccess()
	if j.Count != 0 || j.LastTS != 0 || j.LastError != "" {
		t.Fatalf("journal not cleared: %+v", j)
	}
	if j.ShouldSkip(1, 30*time.Minute) {
		t.Fatal("cleared journal should never skip")
	}
}

func TestJournalRoundTrip(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	path := filepath.Join(dir, "app.selfupdate.json")
	var j Journal
	j.MarkFailure("disk-full")
	j.MarkFailure("disk-full")

	SaveJournal(path, j)
	loaded := LoadJournal(path)
	if loaded.Count != 2 || loaded.LastError != "disk-full" {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
	if !loaded.ShouldSkip(2, 30*time.Minute) {
		t.Fatal("loaded journal should skip after 2 failures")
	}

	// 损坏文件 → 零值（允许重试，不拒绝服务）
	if err := ioutil.WriteFile(path, []byte("not-json"), 0644); err != nil {
		t.Fatalf("write corrupt journal: %v", err)
	}
	loaded = LoadJournal(path)
	if loaded.Count != 0 {
		t.Fatalf("corrupt journal should load as zero value, got %+v", loaded)
	}
}
