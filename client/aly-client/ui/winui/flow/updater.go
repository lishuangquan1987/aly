// +build windows

package flow

// Decide 依据 check_update 结果与现场决定下一步。
//
// 规则（DEVELOPMENT_PLAN §3.5，逐条对应测试）：
//
//  1. Applying（崩溃恢复 / 回滚续跑）→ 无条件 ActionApply，**不询问**。
//     此时主目录可能已被改名甚至丢失，应用处于不可用状态，必须自愈；
//     询问用户会把机器留在坏状态里。
//  2. 无更新 → ActionNone。
//  3. 需要下载 → ActionDownload；force_update 明确为 true 才自动，
//     false 或**缺失（nil）**一律询问。
//  4. 已下载只差应用 → ActionApply；同样只有明确 true 才自动。
//
// 与 C# SDK 的差异（有意）：SDK 把缺失读成 false 且不阻塞，因此强制更新
// 被中断后会静默降级为「询问」；这里在 Applying 分支改为自动续跑。
func Decide(c CheckView) Decision {
	if c.Applying {
		return Decision{Action: ActionApply, Ask: false}
	}
	if !c.HasUpdate {
		return Decision{Action: ActionNone}
	}
	forced := c.IsForced()
	if c.NeedDownload {
		return Decision{Action: ActionDownload, Ask: !forced}
	}
	return Decision{Action: ActionApply, Ask: !forced}
}
