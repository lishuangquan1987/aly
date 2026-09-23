#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""中断场景枚举：对每个流程的每一步注入 Crash，然后跑宿主恢复流程，检查一致性。"""
import copy
import json
from sim import (Env, Version, MAIN, vdir, Crash, cmd_apply_update, cmd_rollback,
                 cmd_download_update, host_recovery, check_consistent)

RESULTS = []


def record(family, crash_step, resume_desc, env, expect=None, note=""):
    ok, desc = check_consistent(env, expect)
    RESULTS.append({
        "family": family,
        "crash_at": crash_step,
        "resume": resume_desc,
        "ok": ok,
        "result": desc,
        "disk": dict(sorted(env.disk.dirs.items())),
        "version": repr(env.ver),
        "note": note,
    })


# ============================================================
# 场景族 A：apply 流程中断（E2E S8 已覆盖，作为基线）
# ============================================================
APPLY_STEPS = ["apply:start", "apply:write_applying", "apply:copy",
               "apply:remove_aside_entry", "apply:aside_prev", "apply:rename_backup",
               "apply:rename_activate", "apply:remove_aside_done", "apply:write_applied"]


def scenario_a():
    for step in APPLY_STEPS:
        # 初始：V1 活动，V2 已下载
        disk = {MAIN: "1.0.0", vdir("2.0.0"): "2.0.0"}
        ver = Version("2.0.0", "1.0.0", "downloaded")
        env = Env(disk, ver, "2.0.0", crash_at=step)
        try:
            cmd_apply_update(env)
        except Crash:
            pass
        # 宿主恢复：check -> (若需要) download -> apply
        env.crash_at = None
        try:
            host_recovery(env)
        except Crash:
            pass
        record("A-apply中断", step, "check→download→apply", env, expect="2.0.0")


# ============================================================
# 场景族 B：rollback 流程中断（E2E 未覆盖）★
# ============================================================
ROLLBACK_STEPS = ["rb:start", "rb:write_applying", "rb:remove_aside_entry",
                  "rb:aside_prev", "rb:rename_backup", "rb:rename_activate",
                  "rb:remove_aside_done"]


def scenario_b_plain():
    """B1: applied 状态下回滚（无待应用版本）"""
    for step in ROLLBACK_STEPS:
        disk = {MAIN: "2.0.0", vdir("1.0.0"): "1.0.0"}
        ver = Version("2.0.0", "1.0.0", "applied")
        env = Env(disk, ver, "2.0.0", crash_at=step)
        try:
            cmd_rollback(env, "1.0.0")
        except Crash:
            pass
        env.crash_at = None
        try:
            host_recovery(env)  # 宿主按常规流程：check→apply
        except Crash:
            pass
        record("B1-rollback中断(无待应用版本)", step, "check→apply", env, expect="1.0.0")


def scenario_b_pending():
    """B2: downloaded 状态（存在待应用新版本 V3）下回滚到 V1，中断★"""
    for step in ROLLBACK_STEPS:
        disk = {MAIN: "2.0.0", vdir("1.0.0"): "1.0.0", vdir("3.0.0"): "3.0.0"}
        ver = Version("3.0.0", "2.0.0", "downloaded")  # V3 已下载待应用
        env = Env(disk, ver, "3.0.0", crash_at=step)
        try:
            cmd_rollback(env, "1.0.0")   # 用户想回滚到 1.0.0
        except Crash:
            pass
        env.crash_at = None
        try:
            host_recovery(env)  # 宿主重启后按常规流程恢复：check→download→apply
        except Crash:
            pass
        record("B2-rollback中断(有待应用版本)", step, "check→download→apply", env,
               expect="1.0.0", note="用户意图=回滚到1.0.0")


def scenario_b_pending_rollback_again():
    """B2': 同上中断，但宿主继续调用 rollback --version 1.0.0（正确用法）"""
    for step in ROLLBACK_STEPS:
        disk = {MAIN: "2.0.0", vdir("1.0.0"): "1.0.0", vdir("3.0.0"): "3.0.0"}
        ver = Version("3.0.0", "2.0.0", "downloaded")
        env = Env(disk, ver, "3.0.0", crash_at=step)
        try:
            cmd_rollback(env, "1.0.0")
        except Crash:
            pass
        env.crash_at = None
        try:
            host_recovery(env, use_rollback="1.0.0")
        except Crash:
            pass
        record("B2'-rollback中断后重跑rollback", step, "rollback --version 1.0.0", env,
               expect="1.0.0")


# ============================================================
# 场景族 C：apply 中断后，宿主先 download 再 apply（download 把 applying 降级）
# ============================================================
def scenario_c():
    for step in APPLY_STEPS:
        disk = {MAIN: "1.0.0", vdir("2.0.0"): "2.0.0"}
        ver = Version("2.0.0", "1.0.0", "downloaded")
        env = Env(disk, ver, "2.0.0", crash_at=step)
        try:
            cmd_apply_update(env)
        except Crash:
            pass
        env.crash_at = None
        try:
            host_recovery(env, always_download=True)  # 宿主无条件先 download
        except Crash:
            pass
        record("C-apply中断后强制download", step, "check→download(强制)→apply", env,
               expect="2.0.0")


# ============================================================
# 场景族 D：download 中断
# ============================================================
DL_STEPS = ["dl:start", "dl:mkdir", "dl:scan", "dl:download", "dl:write_version"]


def scenario_d():
    for step in DL_STEPS:
        disk = {MAIN: "1.0.0"}
        ver = Version("1.0.0", "", "applied")
        env = Env(disk, ver, "2.0.0", crash_at=step)
        try:
            cmd_download_update(env)
        except Crash:
            pass
        env.crash_at = None
        try:
            host_recovery(env)
        except Crash:
            pass
        record("D-download中断", step, "check→download→apply", env, expect="2.0.0")


# ============================================================
# 场景族 E：版本目录残留（中断留下半成品版本目录）→ 后续更新的目录错配
# ============================================================
def scenario_e():
    """V2 下载中断留下半成品目录，V3 发布后下载 V3 并 apply：prevVersionDir 会指向 V2 残留目录"""
    disk = {MAIN: "1.0.0"}
    ver = Version("1.0.0", "", "applied")
    env = Env(disk, ver, "2.0.0")
    # 下载 V2 中途被杀（留下 App_2.0.0 半成品）
    env.crash_at = "dl:write_version"
    try:
        cmd_download_update(env)
    except Crash:
        pass
    env.crash_at = None
    # 服务端发布 V3，客户端下载 V3（VersionPrevious 被改写成 2.0.0）
    env.server_version = "3.0.0"
    r1 = cmd_download_update(env)
    r2 = cmd_apply_update(env)
    ok, desc = check_consistent(env)
    RESULTS.append({
        "family": "E-多次下载导致目录错配",
        "crash_at": "dl:write_version(V2)",
        "resume": "download V3 → apply",
        "ok": ok,
        "result": desc + " | " + str(r2),
        "disk": dict(sorted(env.disk.dirs.items())),
        "version": repr(env.ver),
        "note": "VersionPrevious 被改写成 2.0.0，而 mainFolder 里其实是 1.0.0",
    })


# ============================================================
# 场景族 F：version.json 损坏（断电半写）→ 全命令失败，无自愈
# ============================================================
def scenario_f():
    disk = {MAIN: "2.0.0", vdir("1.0.0"): "1.0.0"}
    ver = Version("2.0.0", "1.0.0", "applied")
    env = Env(disk, ver, "2.0.0")
    # 模拟 version.json 半截 JSON
    env.ver = None  # ReadVersion 返回 error
    RESULTS.append({
        "family": "F-version.json损坏",
        "crash_at": "write_version 期间断电",
        "resume": "check_update / apply_update / rollback",
        "ok": False,
        "result": "ReadVersion 解析失败 → 全部命令 return false，无任何重建/自愈路径，需人工删文件",
        "disk": dict(sorted(env.disk.dirs.items())),
        "version": "损坏（不可解析）",
        "note": "E2E S9a 仅断言『安全报错』，未要求自愈",
    })


for fn in (scenario_a, scenario_b_plain, scenario_b_pending,
           scenario_b_pending_rollback_again, scenario_c, scenario_d,
           scenario_e, scenario_f):
    fn()

bad = [r for r in RESULTS if not r["ok"]]
print("=" * 100)
print("总计场景：%d，不一致/失败：%d" % (len(RESULTS), len(bad)))
print("=" * 100)
for r in RESULTS:
    flag = "PASS" if r["ok"] else "FAIL"
    print("[%s] %-34s crash@%-26s resume=%-28s %s" % (
        flag, r["family"], r["crash_at"], r["resume"], r["result"]))
    if not r["ok"]:
        print("        disk=%s" % r["disk"])
        print("        %s   note=%s" % (r["version"], r["note"]))

with open("result.json", "w", encoding="utf-8") as f:
    json.dump(RESULTS, f, ensure_ascii=False, indent=2)
print("\n明细已写入 result.json")
