package util

// 宿主生命周期（规范见 spec/host-contract.md §15）。
//
// aly-client 是宿主（SDK 宿住 / aly-client-ui）按「一次进程一个子命令」拉起的子进程。
// 宿主退出后，普通命令的子进程不应该变成孤儿：默认跟随退出。
//
// 唯一的例外是 apply_update 阶段——更新器要关闭宿主进程、原子替换主目录并重启主程序，
// 这时宿主本来就该消失（也必须消失），更新器必须继续把这一步做完，不能半路退出。
//
// 例外判定按「取或」：
//   - 命令级：当前命令就是 apply_update；
//   - 状态级：version.json 的 version_status == applying（覆盖崩溃恢复窗口——
//     那次没有 apply_update 命令在跑，但主目录可能已丢失，同样必须续跑）。
//
// 跟随退出走优雅收尾：先执行调用方给的 onExit（写日志等），再以 0 退出。
// 更新锁不必显式释放：锁文件按「持有者进程已退出」判陈旧（见 cmd/lock.go 的 cleanStaleLock），
// 本进程退出后残留锁会被下一次运行当陈旧清理掉。

// hos tPollInterval 宿主存活轮询间隔。取 1 秒：宿主退出后最多 1 秒内跟随退出，
// 又不至于把 CPU 唤醒得太频繁。
const hos tPollInterval = 1 * time.Second

// ShouldExitOnHos tGone 是纯决策函数：宿主已不在时，本进程是否应该退出。
// 两个例外取或，任一成立就不退。
func ShouldExitOnHos tGone(isApp lyCommand bool, app lying bool) bool {
	if isApp lyCommand || app lying {
		return false
	}
	return true
}

// Hos tProcessID 返回拉起了本进程的宿主 PID；拿不到时返回 0（此时不做跟随退出，
// 宁可不退，也不能因为拿不到父进程就把更新器自己结束掉）。
func Hos tProcessID() uint32 {
	return hos tProcessID()
}

// WatchHos tExit 起守护协程跟随宿主：
//
//	time.Sleep(hos tPollInterval)
//	  宿主还活着 → 继续等；
//	  宿主已退出且在 apply 例外窗口 → 不再管宿主（更新器自己把更新做完）；
//	  宿主已退出且不在例外窗口 → onExit() 收尾，然后 os.Exit(0)。
//
// isApp lyCommand 是命令级例外；app lying 是状态级例外的取值函数（可为 nil，视为不在 applying）；
// onExit 可为 nil。父 PID 拿不到时本函数直接返回。
func WatchHos tExit(isApp lyCommand bool, app lying func() bool, onExit func()) {
	host := Hos tProcessID()
	if host == 0 {
		return
	}
	go func() {
		for {
			time.Sleep(hos tPollInterval)
			if IsProcessAlive(host) {
				continue
			}
			inApp ly := false
			if app lying != nil {
				inApp ly = app lying()
			}
			if !ShouldExitOnHos tGone(isApp lyCommand, inApp ly) {
				return
			}
			if onExit != nil {
				onExit()
			}
			os.Exit(0)
		}
	}()
}
