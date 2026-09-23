#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""多轮恢复：SDK MainLoop 每 5s 轮询一次（check→[download]→apply），
因此一次失败不算终局 —— 需要跑到状态收敛，才能区分『可自愈』与『死局』。"""
import json
from sim import (Env, Version, MAIN, vdir, Crash, cmd_apply_update, cmd_rollback,
                 cmd_download_update, cmd_check_update, check_consistent)

MAX_ROUNDS = 4

APPLY_STEPS = ["apply:start", "apply:write_applying", "apply:copy",
               "apply:remove_aside_entry", "apply:aside_prev", "apply:rename_backup",
               "apply:rename_activate", "apply:remove_aside_done", "apply:write_applied"]
ROLLBACK_STEPS = ["rb:start", "rb:write_applying", "rb:remove_aside_entry",
                  "rb:aside_prev", "rb:rename_backup", "rb:rename_activate",
                  "rb:remove_aside_done"]


def run_rounds(env, force_download=False, rollback_to=None):
    """模拟宿主多轮轮询，返回 (轮数, 最后一轮结果, 是否收敛到 applied)"""
    last = ""
    for r in range(1, MAX_ROUNDS + 1):
        before = repr(env.ver)
        try:
            if rollback_to:
                last = cmd_rollback(env, rollback_to)
            else:
                need, nv = cmd_check_update(env)
                if need or force_download:
                    cmd_download_update(env)
                last = cmd_apply_update(env)
        except Crash:
            last = "CRASH"
        after = repr(env.ver)
        if env.ver.VersionStatus == "applied" or before == after:
            return r, last, env.ver.VersionStatus == "applied"
    return MAX_ROUNDS, last, env.ver.VersionStatus == "applied"


def scenario(family, steps, cmd, init_disk, init_ver, server_after=None,
             expect=None, resume_desc="", force_download=False, rollback_to=None,
             server_final=None):
    out = []
    for step in steps:
        env = Env(dict(init_disk), init_ver.copy(), server_final or init_ver.Version,
                  crash_at=step)
        try:
            cmd(env)
        except Crash:
            pass
        env.crash_at = None
        if server_after:
            env.server_version = server_after
        rounds, last, converged = run_rounds(env, force_download, rollback_to)
        ok, desc = check_consistent(env, expect)
        out.append({
            "family": family, "crash_at": step, "rounds": rounds, "converged": converged,
            "last": last, "ok": ok, "result": desc,
            "disk": dict(sorted(env.disk.dirs.items())), "version": repr(env.ver),
            "resume": resume_desc,
        })
    return out


RESULTS = []

# A: apply 中断，服务器版本不变（SDK 不 download，直接 apply）
RESULTS += scenario("A-apply中断(服务器不变)", APPLY_STEPS, cmd_apply_update,
                    {MAIN: "1.0.0", vdir("2.0.0"): "2.0.0"},
                    Version("2.0.0", "1.0.0", "downloaded"),
                    expect="2.0.0", resume_desc="check→apply（SDK 默认）")

# G: apply 中断，期间服务器发新版 V3（SDK 会 download）
RESULTS += scenario("G-apply中断+服务器发V3", APPLY_STEPS, cmd_apply_update,
                    {MAIN: "1.0.0", vdir("2.0.0"): "2.0.0"},
                    Version("2.0.0", "1.0.0", "downloaded"),
                    server_after="3.0.0", expect="3.0.0",
                    resume_desc="check→download(V3)→apply", server_final="3.0.0")

# C: apply 中断，宿主无条件 download（服务器版本没变）
RESULTS += scenario("C-apply中断+强制download", APPLY_STEPS, cmd_apply_update,
                    {MAIN: "1.0.0", vdir("2.0.0"): "2.0.0"},
                    Version("2.0.0", "1.0.0", "downloaded"),
                    expect="2.0.0", resume_desc="check→download(无条件)→apply",
                    force_download=True)

# B1: rollback 中断（无待应用版本），宿主按 SDK 流程 check→apply
RESULTS += scenario("B1-rollback中断(无待应用版本)", ROLLBACK_STEPS,
                    lambda e: cmd_rollback(e, "1.0.0"),
                    {MAIN: "2.0.0", vdir("1.0.0"): "1.0.0"},
                    Version("2.0.0", "1.0.0", "applied"),
                    expect="1.0.0", resume_desc="check→apply（SDK 不会续跑 rollback）")

# B2: rollback 中断（存在待应用 V3），宿主按 SDK 流程
RESULTS += scenario("B2-rollback中断(有待应用V3)", ROLLBACK_STEPS,
                    lambda e: cmd_rollback(e, "1.0.0"),
                    {MAIN: "2.0.0", vdir("1.0.0"): "1.0.0", vdir("3.0.0"): "3.0.0"},
                    Version("3.0.0", "2.0.0", "downloaded"),
                    expect="1.0.0", resume_desc="check→download→apply（用户意图=回滚到1.0.0）")

# B2': rollback 中断后用户重试 rollback
RESULTS += scenario("B2'-rollback中断后重跑rollback", ROLLBACK_STEPS,
                    lambda e: cmd_rollback(e, "1.0.0"),
                    {MAIN: "2.0.0", vdir("1.0.0"): "1.0.0", vdir("3.0.0"): "3.0.0"},
                    Version("3.0.0", "2.0.0", "downloaded"),
                    expect="1.0.0", resume_desc="rollback --version 1.0.0",
                    rollback_to="1.0.0")

# H: rollback 中断 + 期间服务器发新版（B2 + G 组合）：
# check 必须优先续回滚（RollbackPrevious 分支），绝不因服务器 V4 转向下载/升级。
RESULTS += scenario("H-回滚中断+服务器发V4", ROLLBACK_STEPS,
                    lambda e: cmd_rollback(e, "1.0.0"),
                    {MAIN: "2.0.0", vdir("1.0.0"): "1.0.0", vdir("3.0.0"): "3.0.0"},
                    Version("3.0.0", "2.0.0", "downloaded"),
                    server_after="4.0.0", expect="1.0.0",
                    resume_desc="check→download→apply（用户意图=回滚到1.0.0，服务器已发V4）",
                    server_final="4.0.0")

print("=" * 108)
print("多轮恢复最终判定（最多 %d 轮）：共 %d 场景" % (MAX_ROUNDS, len(RESULTS)))
print("=" * 108)
cur = None
for r in RESULTS:
    if r["family"] != cur:
        cur = r["family"]
        print("\n--- %s ---" % cur)
    flag = "PASS" if r["ok"] else ("FAIL" if not r["converged"] else "WARN")
    print("  [%s] crash@%-22s %d轮 %-34s %s" % (
        flag, r["crash_at"], r["rounds"], r["result"][:34], "" if r["ok"] else r["last"]))
    if not r["ok"]:
        print("         disk=%s" % r["disk"])
        print("         %s" % r["version"])

with open("result_final.json", "w", encoding="utf-8") as f:
    json.dump(RESULTS, f, ensure_ascii=False, indent=2)
print("\n明细已写入 result_final.json")
