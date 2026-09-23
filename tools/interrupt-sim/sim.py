#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
aly-client 更新状态机「中断 → 恢复」模拟器

忠实复刻 client/aly-client 以下代码的核心语义（省略进程查杀/网络/日志等非状态因素）：
  - cmd/apply_update.go    ApplyUpdate / applyReplacement
  - cmd/rollback.go        Rollback
  - cmd/download_update.go DownloadUpdate
  - cmd/check_update.go    CheckUpdate
  - cmd/common.go          applyFailureFallback / renameDirWithKillState(旁移)
  - config/version.go      ReadVersion / WriteVersion

磁盘模型：PackageFolder 下的目录名 -> 该目录"内容所属版本"。
  MainFolder = "App"（活动目录）
  版本目录   = "App_" + version
  旁移残留   = "App_" + version + ".old" / ".old.N"

一致性判据（恢复是否成功）：
  1. MainFolder 必须存在（否则应用打不开）
  2. status=applied 时：MainFolder 内容版本 == version.json.Version
  3. status=downloaded 时：MainFolder 内容版本 == version.json.VersionPrevious
  4. 回滚目录 App_{VersionPrevious} 若存在，其内容版本应 == VersionPrevious（否则回滚拿到错的内容）
"""
import copy
import itertools

MAIN = "App"


def vdir(v):
    return "App_" + v


class Crash(Exception):
    """模拟断电 / 任务管理器结束进程：进程瞬间消失，磁盘保持当前状态"""
    pass


class RenameErr(Exception):
    pass


class Disk:
    def __init__(self, dirs):
        self.dirs = dict(dirs)  # name -> content version

    def exists(self, p):
        return p in self.dirs

    def rename(self, a, b):
        """os.Rename 语义：源必须存在；目标若已存在（非空目录）在 Windows 上失败"""
        if a not in self.dirs:
            raise RenameErr("rename %s -> %s: source not found" % (a, b))
        if b in self.dirs:
            raise RenameErr("rename %s -> %s: target exists (access denied)" % (a, b))
        self.dirs[b] = self.dirs.pop(a)

    def remove_aside_variants(self, base):
        """common.go removeAsideVariants：删除 base.old / base.old.1 ... base.old.64"""
        for c in [base + ".old"] + ["%s.old.%d" % (base, i) for i in range(1, 65)]:
            self.dirs.pop(c, None)

    def snapshot(self):
        return dict(self.dirs)


class Version:
    def __init__(self, version="", previous="", status="", rollback_previous="",
                 after_script=""):
        self.Version = version
        self.VersionPrevious = previous
        self.VersionStatus = status
        self.RollbackPrevious = rollback_previous
        self.AfterApplyUpdateScript = after_script

    def copy(self):
        return Version(self.Version, self.VersionPrevious, self.VersionStatus,
                       self.RollbackPrevious, self.AfterApplyUpdateScript)

    def __repr__(self):
        return "Version=%s Prev=%s Status=%s RollbackPrev=%s" % (
            self.Version, self.VersionPrevious, self.VersionStatus, self.RollbackPrevious or "-")


class Env:
    """运行时环境：磁盘 + version.json + 服务器版本 + 断点"""

    def __init__(self, disk, ver, server_version, crash_at=None):
        self.disk = Disk(disk)
        self.ver = ver
        self.server_version = server_version
        self.crash_at = crash_at  # (op, step) 命中即抛 Crash
        self.log = []

    # ---- 断点工具 ----
    def tick(self, step):
        self.log.append(step)
        if self.crash_at is not None and self.crash_at == step:
            raise Crash(step)

    # ---- 配置派生 ----
    def main_folder(self):
        return MAIN

    def version_dir(self, v):
        return vdir(v)


# ============================================================
# 命令实现（严格对应 Go 代码）
# ============================================================

def cmd_download_update(env):
    """download_update.go"""
    vi = env.ver
    env.tick("dl:start")
    new_version = env.server_version

    # 守卫（71-81 行）
    if vi.VersionStatus != "applying":
        if vi.VersionStatus == "downloaded" and vi.Version == new_version:
            return "skip: already downloaded"
        if cmp_version(new_version, vi.Version) <= 0:
            return "skip: already latest"

    target = env.version_dir(new_version)
    env.tick("dl:mkdir")
    env.disk.dirs.setdefault(target, new_version)  # 目录创建（内容视为新版本）

    # localMD5Map(MainFolder)；MainFolder 不存在时返回空 map + error（代码只记日志继续）
    env.tick("dl:scan")
    local_ok = env.disk.exists(MAIN)

    # 下载（此处简化为：MainFolder 缺失 -> 需下载全量；否则只下载差异）
    env.tick("dl:download")
    # 目标目录内容标记为 new_version（无论下载多少文件，最终校验后都是新版本内容）
    env.disk.dirs[target] = new_version

    env.tick("dl:write_version")
    # 196-204 行：注意 VersionPrevious 被无条件改写成旧 Version
    vi.VersionPrevious = vi.Version
    vi.Version = new_version
    vi.VersionStatus = "downloaded"
    vi.RollbackPrevious = ""
    return "downloaded"


def cmd_check_update(env):
    """check_update.go：返回 (need_download, new_version)"""
    vi = env.ver
    if vi.VersionStatus in ("", "applied"):
        if cmp_version(env.server_version, vi.Version) > 0:
            return True, env.server_version
        return False, vi.Version
    # downloaded / applying
    if cmp_version(env.server_version, vi.Version) > 0:
        return True, env.server_version
    return False, vi.Version  # 继续 apply 已下载版本


def _apply_replacement(env, vi, version_dir):
    """apply_update.go applyReplacement（212-289 行）"""
    env.tick("apply:copy")
    # 真实代码：filepath.Walk(mainFolder) 在源目录不存在时返回 lstat 错误
    # -> applyReplacement 直接 return "copy to version dir: ..."，不会走到改名
    if not env.disk.exists(MAIN):
        raise RenameErr("copy to version dir: %s not found" % MAIN)
    if not env.disk.exists(version_dir):
        raise RenameErr("version dir missing")
    # 复制只补缺，不覆盖已有（已下载的新版本文件保持）
    if env.disk.dirs[version_dir] != vi.Version:
        env.disk.dirs[version_dir] = vi.Version  # 补齐未变更文件后整体成为新版本内容

    prev_dir = env.version_dir(vi.VersionPrevious)
    old_backup_temp = prev_dir + ".old"

    env.tick("apply:remove_aside_entry")
    env.disk.remove_aside_variants(prev_dir)  # ← 入口无条件清理 .old 系列

    if env.disk.exists(prev_dir):
        env.tick("apply:aside_prev")
        env.disk.rename(prev_dir, old_backup_temp)
    env.tick("apply:rename_backup")
    env.disk.rename(MAIN, prev_dir)
    env.tick("apply:rename_activate")
    env.disk.rename(version_dir, MAIN)
    env.tick("apply:remove_aside_done")
    env.disk.remove_aside_variants(prev_dir)


def cmd_apply_update(env):
    """apply_update.go"""
    vi = env.ver
    env.tick("apply:start")

    if vi.VersionStatus == "applied":
        return "no pending update to apply"

    if vi.VersionStatus == "applying":
        vd = env.version_dir(vi.Version)
        if env.disk.exists(MAIN):
            if not env.disk.exists(vd):
                # 分支 1：认为新版本已就位（78-106 行）
                env.tick("apply:recover_mark_applied")
                vi.VersionStatus = "applied"
                vi.RollbackPrevious = ""
                return "recovered: marked applied"
            # versionDir 仍存在 -> fall through 重做替换
        else:
            if env.disk.exists(vd):
                env.tick("apply:recover_rename")
                env.disk.rename(vd, MAIN)
                vi.VersionStatus = "applied"
                vi.RollbackPrevious = ""
                return "recovered: renamed versionDir -> MainFolder"
            return "CRASH RECOVERY FAILED: neither main nor version dir exists"

    # status == downloaded（或 fall through）
    env.tick("apply:write_applying")
    vi.VersionStatus = "applying"

    vd = env.version_dir(vi.Version)
    try:
        _apply_replacement(env, vi, vd)
    except (RenameErr, Crash) as e:
        if isinstance(e, Crash):
            raise
        # apply_failure_fallback
        if not env.disk.exists(MAIN) and env.disk.exists(vd):
            return "FAILED (keep applying): %s" % e
        vi.VersionStatus = "downloaded"
        return "FAILED (downgraded): %s" % e

    env.tick("apply:write_applied")
    vi.VersionStatus = "applied"
    vi.RollbackPrevious = ""
    return "applied"


def cmd_rollback(env, target):
    """rollback.go"""
    vi = env.ver
    env.tick("rb:start")
    vd = env.version_dir(target)
    if not env.disk.exists(vd):
        return "version %s not found" % target

    if vi.VersionStatus == "downloaded" and target == vi.Version:
        return "不可回滚到未应用的下载版本"

    old_version = vi.Version
    if vi.VersionStatus == "downloaded" and vi.VersionPrevious:
        old_version = vi.VersionPrevious
    if vi.VersionStatus == "applying" and vi.RollbackPrevious:
        old_version = vi.RollbackPrevious
    if target == old_version:
        return "当前已处于版本 %s" % target

    if vi.VersionStatus == "applying":
        if env.disk.exists(MAIN):
            pass  # fall through 重做
        else:
            if env.disk.exists(vd):
                env.tick("rb:recover_rename")
                env.disk.rename(vd, MAIN)
                vi.Version = target
                vi.VersionPrevious = old_version
                vi.VersionStatus = "applied"
                vi.RollbackPrevious = ""
                return "recovered: renamed %s -> MainFolder" % vd
            return "CRASH RECOVERY FAILED"

    env.tick("rb:write_applying")
    vi.VersionStatus = "applying"
    vi.RollbackPrevious = old_version

    prev_dir = env.version_dir(old_version)
    old_backup_temp = prev_dir + ".old"

    env.tick("rb:remove_aside_entry")
    env.disk.remove_aside_variants(prev_dir)

    try:
        if env.disk.exists(prev_dir):
            env.tick("rb:aside_prev")
            env.disk.rename(prev_dir, old_backup_temp)
        env.tick("rb:rename_backup")
        env.disk.rename(MAIN, prev_dir)
        env.tick("rb:rename_activate")
        env.disk.rename(vd, MAIN)
    except (RenameErr, Crash) as e:
        if isinstance(e, Crash):
            raise
        # 失败路径：状态一律写回 applied + 清空 RollbackPrevious（rollback.go 各失败分支）
        vi.VersionStatus = "applied"
        vi.RollbackPrevious = ""
        return "FAILED: %s" % e

    env.tick("rb:remove_aside_done")
    env.disk.remove_aside_variants(prev_dir)
    vi.VersionPrevious = old_version
    vi.Version = target
    vi.VersionStatus = "applied"
    vi.RollbackPrevious = ""
    return "rolled back to %s" % target


def cmp_version(a, b):
    def parts(s):
        out = []
        for p in s.split("."):
            try:
                out.append(int(p))
            except ValueError:
                out.append(0)
        return out
    pa, pb = parts(a), parts(b)
    n = max(len(pa), len(pb))
    pa += [0] * (n - len(pa))
    pb += [0] * (n - len(pb))
    return (pa > pb) - (pa < pb)


# ============================================================
# 一致性检查
# ============================================================

def check_consistent(env, expect_version=None):
    """返回 (ok, 说明)"""
    vi = env.ver
    if not env.disk.exists(MAIN):
        return False, "MainFolder 缺失（应用不可用）"
    content = env.disk.dirs[MAIN]
    if vi.VersionStatus == "applied":
        want = vi.Version
    elif vi.VersionStatus == "downloaded":
        want = vi.VersionPrevious
    else:
        return False, "status 停在 %s（未完成恢复）" % vi.VersionStatus
    if content != want:
        return False, "版本错配：磁盘内容是 %s，version.json 声明 %s" % (content, want)
    if expect_version is not None and content != expect_version:
        return False, "意图错乱：期望 %s，实际活动 %s" % (expect_version, content)
    # 回滚目录一致性
    pv = env.version_dir(vi.VersionPrevious)
    if env.disk.exists(pv) and env.disk.dirs[pv] != vi.VersionPrevious:
        return False, "回滚目录 %s 装的是 %s，名字却是 %s（回滚会拿到错内容）" % (
            pv, env.disk.dirs[pv], vi.VersionPrevious)
    return True, "OK（活动版本 %s）" % content


def host_recovery(env, use_rollback=None, always_download=False):
    """宿主程序的常规恢复流程：check_update -> [download] -> apply_update"""
    need_dl, nv = cmd_check_update(env)
    if use_rollback:
        return cmd_rollback(env, use_rollback)
    if need_dl or always_download:
        cmd_download_update(env)
    return cmd_apply_update(env)
