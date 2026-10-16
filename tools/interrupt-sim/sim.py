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

已建模（2026-10 状态机加固后同步）：
  - F1/F6  apply：空状态（"")与未知 status 一律拒绝替换（此前会 fall through 成"自改名假成功"）
  - F3/F4  apply：downloaded 现场校验 —— 版本目录缺失/为空 → 拒绝并自愈为 applied+VersionPrevious；
           Version == VersionPrevious → 拒绝（check 不再上报，避免反复杀进程）
  - F5     apply：进入"激活改名"前写 apply_stage=activating，作为崩溃恢复"改名确实发生过"的证据
  - F2     首次更新（VersionPrevious 为空）成功后删除无名备份目录 App_（不可回滚、不会被 prune 清理）
  - F7     所有终态写入清空 RollbackPrevious/RollbackTarget/ApplyStage
  - F8     rollback 失败恢复"回滚前状态"（不再一律写 applied）
  - F9     status=applying 且 rollback_previous 为空（未完成的 apply）→ 拒绝 rollback

未建模（模型外，改动本身有 Go 单测/E2E 覆盖）：
  - 服务器不可达（check_update 断网语义；本模型假定 server_version 始终可知）
  - check_update 输出字段 current_version 的口径（模型只关心 need_download/new_version）
  - 进程查杀、日志、锁、后置脚本

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
                 after_script="", rollback_target="", apply_stage=""):
        self.Version = version
        self.VersionPrevious = previous
        self.VersionStatus = status
        self.RollbackPrevious = rollback_previous
        self.RollbackTarget = rollback_target
        self.AfterApplyUpdateScript = after_script
        # F5：仅 apply 在途（applying）期间写入，终态一律清空
        self.ApplyStage = apply_stage

    def copy(self):
        return Version(self.Version, self.VersionPrevious, self.VersionStatus,
                       self.RollbackPrevious, self.AfterApplyUpdateScript, self.RollbackTarget,
                       self.ApplyStage)

    def __repr__(self):
        return "Version=%s Prev=%s Status=%s RollbackPrev=%s RollbackTarget=%s" % (
            self.Version, self.VersionPrevious, self.VersionStatus, self.RollbackPrevious or "-",
            self.RollbackTarget or "-")


class Env:
    """运行时环境：磁盘 + version.json + 服务器版本 + 断点"""

    def __init__(self, disk, ver, server_version, crash_at=None):
        self.disk = Disk(disk)
        self.ver = ver
        self.server_version = server_version
        self.crash_at = crash_at  # (op, step) 命中即抛 Crash
        self.log = []
        # F4：check 判定"状态非法、无动作可做"时置位，宿主本轮不再 apply/download
        self.host_idle = False

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

    # applying 期间：不下载、不改写版本状态，由 apply_update 恢复在途操作（修复 Bug#3）。
    # download 不得把 applying 降级成 downloaded、不得覆盖 VersionPrevious/RollbackPrevious。
    if vi.VersionStatus == "applying":
        return "skipped: applying in progress"

    # F4：非法状态（downloaded 且 VersionPrevious == Version）→ 归一化为 applied；
    # B1：仅当服务器没有更高版本时才归一化，否则会把"下载更高版本"的请求变成空操作
    if (vi.VersionStatus == "downloaded" and vi.VersionPrevious
            and vi.VersionPrevious == vi.Version
            and cmp_version(new_version, vi.Version) <= 0):
        vi.VersionStatus = "applied"
        vi.RollbackPrevious = ""
        vi.RollbackTarget = ""
        vi.ApplyStage = ""
        return "normalized: illegal state -> applied"

    # 守卫（applying 已在上面早退，此处无需再保留例外）
    re_download = False
    if vi.VersionStatus == "downloaded" and vi.Version == new_version:
        if env.disk.exists(env.version_dir(new_version)):
            return "skip: already downloaded"
        # F3：产物丢失 → 允许"重下同版本"补齐（需旁路下面的"已是最新"判断）
        re_download = True
    if not re_download and cmp_version(new_version, vi.Version) <= 0:
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
    # 195-204 行：VersionPrevious 仅在可确定时更新（修复 Bug#2）：
    #   downloaded → 保持不变（MainFolder 仍是 VersionPrevious 的内容）
    #   applied / 空 → VersionPrevious = Version（当前活动内容版本）
    if vi.VersionStatus != "downloaded":
        vi.VersionPrevious = vi.Version
    vi.Version = new_version
    vi.VersionStatus = "downloaded"
    vi.RollbackPrevious = ""
    vi.RollbackTarget = ""
    vi.ApplyStage = ""  # F5/F7：新的待应用现场，清掉上一轮的过渡字段
    return "downloaded"


def cmd_check_update(env):
    """check_update.go：返回 (need_download, new_version)"""
    vi = env.ver
    # 回滚中断：优先完成回滚，绝不因服务器发布新版本而转向下载/升级（修复 Bug#1/#5）
    if vi.RollbackPrevious:
        target = vi.RollbackTarget or vi.Version
        return False, target
    if vi.VersionStatus in ("", "applied"):
        if cmp_version(env.server_version, vi.Version) > 0:
            return True, env.server_version
        return False, vi.Version
    # downloaded / applying
    if vi.VersionStatus == "applying":
        # applying（更新中断，非回滚）：先完成在途 apply，不转向重下（修复 Bug#3）
        return False, vi.Version
    if cmp_version(env.server_version, vi.Version) > 0:
        return True, env.server_version
    # F3：待应用版本目录缺失 → 仅当服务器"正好是该待应用版本"时要求重下同版本补齐；
    # 服务器更低（B3：download 会因 already at latest 拒绝）时落到"继续 apply"，由 apply 自愈
    if not env.disk.exists(env.version_dir(vi.Version)) and env.server_version == vi.Version:
        return True, vi.Version
    # F4：非法状态（Version == VersionPrevious，备份目录与版本目录同路径）→ apply 会拒绝；
    # check 不上报待应用更新，避免宿主反复 apply → 拒绝 → 兜底回 downloaded 的杀进程循环。
    if vi.VersionPrevious and vi.VersionPrevious == vi.Version:
        env.host_idle = True
        return False, vi.Version
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

    # 防御：Version == VersionPrevious（异常/遗留状态）→ 拒绝替换（Bug#3 变体：
    # 备份目录与版本目录同路径，旁移-备份-激活会自毁成静默装回旧版本）
    if vi.VersionPrevious and vi.VersionPrevious == vi.Version:
        raise RenameErr("reject: Version == VersionPrevious(%s)" % vi.Version)

    env.tick("apply:remove_aside_entry")
    # 加固①：不再在入口清理 .old 残留（崩溃点"旁移后/备份改名前"时该残留是尚未被
    # 本次替换覆盖的历史快照），清理时机后移到成功路径（apply:remove_aside_done）。
    # env.disk.remove_aside_variants(prev_dir)  ← 已移除

    if env.disk.exists(prev_dir):
        env.tick("apply:aside_prev")
        env.disk.rename(prev_dir, old_backup_temp)
    env.tick("apply:rename_backup")
    env.disk.rename(MAIN, prev_dir)
    # F5：进入"激活改名"前写 apply_stage=activating（崩溃恢复的证据）
    vi.ApplyStage = "activating"
    env.tick("apply:rename_activate")
    env.disk.rename(version_dir, MAIN)
    env.tick("apply:remove_aside_done")
    env.disk.remove_aside_variants(prev_dir)


def cmd_apply_update(env):
    """apply_update.go"""
    vi = env.ver
    env.tick("apply:start")

    if vi.VersionStatus in ("", "applied"):
        # F1：空状态（version.json 缺失/损坏自愈重置）与 applied 同义，拒绝替换
        return "no pending update to apply"
    if vi.VersionStatus not in ("applying", "downloaded"):
        # F6：未知 status 不猜测、不替换（只允许 download_update 重写成 downloaded 修复）
        return "unknown version_status: refuse to apply"

    if vi.VersionStatus == "applying":
        # 回滚中断：按回滚语义恢复（修复 Bug#1/#5），绝不按 vi.Version 升级
        if vi.RollbackPrevious:
            return cmd_resume_rollback(env, vi.RollbackTarget)
        vd = env.version_dir(vi.Version)
        if env.disk.exists(MAIN):
            if not env.disk.exists(vd):
                # 分支 1：认为新版本已就位（78-106 行）。
                # F5：Go 侧此时会检查 apply_stage==activating 作为"改名确实发生过"的证据，
                # 缺证据（老现场）退回历史启发式并写 update.log；两条路径结果都是补写 applied，
                # 故模型不区分，只做过渡字段清理。
                env.tick("apply:recover_mark_applied")
                vi.VersionStatus = "applied"
                vi.RollbackPrevious = ""
                vi.RollbackTarget = ""
                vi.ApplyStage = ""
                if not vi.VersionPrevious and vi.Version:
                    env.disk.dirs.pop(vdir(""), None)  # F2：清理无名备份目录
                return "recovered: marked applied"
            # versionDir 仍存在 -> fall through 重做替换
        else:
            if env.disk.exists(vd):
                env.tick("apply:recover_rename")
                env.disk.rename(vd, MAIN)
                vi.VersionStatus = "applied"
                vi.RollbackPrevious = ""
                vi.RollbackTarget = ""
                vi.ApplyStage = ""
                if not vi.VersionPrevious and vi.Version:
                    env.disk.dirs.pop(vdir(""), None)  # F2：清理无名备份目录
                return "recovered: renamed versionDir -> MainFolder"
            return "CRASH RECOVERY FAILED: neither main nor version dir exists"

    # status == downloaded：替换前校验现场（A1/F3/F4），拒绝路径不写状态、不改名
    if not vi.Version:
        # A1：Version 为空时 AppVersionDir("") 退化成无名备份目录，不得当待应用版本
        vi.Version = vi.VersionPrevious
        vi.VersionStatus = "applied"
        vi.RollbackPrevious = ""
        vi.RollbackTarget = ""
        vi.ApplyStage = ""
        return "refuse: empty version -> repaired to applied/%s" % vi.Version
    if vi.VersionPrevious and vi.VersionPrevious == vi.Version:
        return "refuse: Version == VersionPrevious(%s)" % vi.Version
    vd = env.version_dir(vi.Version)
    if not env.disk.exists(vd):
        # F3：待应用版本目录缺失 → 拒绝替换（否则自拷贝假成功），并自愈为 applied+VersionPrevious
        vi.RollbackPrevious = ""
        vi.RollbackTarget = ""
        vi.ApplyStage = ""
        vi.Version = vi.VersionPrevious
        vi.VersionStatus = "applied"
        return "refuse: pending version dir missing -> repaired to applied/%s" % vi.Version

    env.tick("apply:write_applying")
    vi.VersionStatus = "applying"
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
    vi.RollbackTarget = ""
    vi.ApplyStage = ""
    if not vi.VersionPrevious and vi.Version:
        # F2：首次更新（无上一版本）不保留无名备份目录 App_
        env.disk.dirs.pop(vdir(""), None)
    return "applied"


def cmd_rollback(env, target):
    """rollback.go"""
    vi = env.ver
    env.tick("rb:start")

    # 回滚中断续跑：目标目录可能已被消耗（激活已完成）→ 委托崩溃恢复（修复 Bug#1 B2'），
    # 不再以 "version not found" 拒绝。
    if vi.VersionStatus == "applying" and vi.RollbackPrevious:
        return cmd_resume_rollback(env, vi.RollbackTarget or target)

    # F6：未知 version_status 不猜测、不替换（与 apply 口径一致；只有 download 能重写修复）
    if vi.VersionStatus not in ("", "applied", "downloaded", "applying"):
        return "unknown version_status: refuse to rollback"

    # F9：status=applying 且 rollback_previous 为空 = 有一笔未完成的 apply；
    # 此时回滚会把待应用升级包旁移删除、并把主目录旧内容改名成 App_{Version}（目录名与内容错配）
    if vi.VersionStatus == "applying" and not vi.RollbackPrevious:
        return "refuse: apply in progress (run apply_update first)"

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
                vi.RollbackTarget = ""
                return "recovered: renamed %s -> MainFolder" % vd
            return "CRASH RECOVERY FAILED"

    # F8：记住回滚前状态，失败时恢复它（不再一律写 applied）
    pre_status = vi.VersionStatus

    env.tick("rb:write_applying")
    vi.VersionStatus = "applying"
    vi.RollbackPrevious = old_version
    vi.RollbackTarget = target

    prev_dir = env.version_dir(old_version)
    old_backup_temp = prev_dir + ".old"

    # 加固①：不再在入口清理 .old 残留，清理时机后移到成功路径（rb:remove_aside_done）

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
        # 失败路径：F8 恢复回滚前状态 + 清空过渡字段（rollback.go 各失败分支）
        vi.VersionStatus = pre_status
        vi.RollbackPrevious = ""
        vi.RollbackTarget = ""
        vi.ApplyStage = ""
        return "FAILED: %s" % e

    env.tick("rb:remove_aside_done")
    env.disk.remove_aside_variants(prev_dir)
    vi.VersionPrevious = old_version
    vi.Version = target
    vi.VersionStatus = "applied"
    vi.RollbackPrevious = ""
    vi.RollbackTarget = ""
    vi.ApplyStage = ""
    if not vi.VersionPrevious and vi.Version:
        env.disk.dirs.pop(vdir(""), None)  # F2：清理无名备份目录
    return "rolled back to %s" % target


def cmd_resume_rollback(env, target):
    """rollback.go resumeRollback：完成一次被中断的回滚（status=applying && rollback_previous != ""）。
    按磁盘现场 4 分支恢复，只做重命名；绝不把回滚当成升级处理。"""
    vi = env.ver
    old_version = vi.RollbackPrevious

    # 无法确定目标（老数据无 rollback_target 且无 CLI 目标）：安全放弃回滚，
    # 恢复/归位到回滚前版本，绝不升级（修复 Bug#1 兜底）。
    if not target:
        if not env.disk.exists(MAIN):
            prev_dir = env.version_dir(old_version)
            if env.disk.exists(prev_dir):
                env.disk.rename(prev_dir, MAIN)
        vi.Version = old_version
        vi.VersionPrevious = old_version
        vi.VersionStatus = "applied"
        vi.RollbackPrevious = ""
        vi.RollbackTarget = ""
        return "aborted rollback, restored to %s" % old_version

    vd = env.version_dir(target)
    prev_dir = env.version_dir(old_version)

    if env.disk.exists(MAIN):
        # MainFolder 存在
        if not env.disk.exists(vd):
            # 分支 1：激活已完成（目标目录已消耗）→ 补写 applied
            vi.Version = target
            vi.VersionPrevious = old_version
            vi.VersionStatus = "applied"
            vi.RollbackPrevious = ""
            vi.RollbackTarget = ""
            return "recovered: rollback already applied (%s)" % target
        # 分支 2：备份改名前的崩溃 → 重做回滚替换（仅重命名）
        env.disk.remove_aside_variants(prev_dir)
        old_backup_temp = prev_dir + ".old"
        if env.disk.exists(prev_dir):
            env.disk.rename(prev_dir, old_backup_temp)
        env.disk.rename(MAIN, prev_dir)
        env.disk.rename(vd, MAIN)
        env.disk.remove_aside_variants(prev_dir)
        vi.Version = target
        vi.VersionPrevious = old_version
        vi.VersionStatus = "applied"
        vi.RollbackPrevious = ""
        vi.RollbackTarget = ""
        return "recovered: redone rollback swap (%s)" % target

    # MainFolder 缺失
    if env.disk.exists(vd):
        # 分支 3：备份改名后、激活前崩溃 → 激活目标
        env.disk.rename(vd, MAIN)
        vi.Version = target
        vi.VersionPrevious = old_version
        vi.VersionStatus = "applied"
        vi.RollbackPrevious = ""
        vi.RollbackTarget = ""
        return "recovered: activated %s" % target
    if env.disk.exists(prev_dir):
        # 分支 4：目标已消耗（B2'）→ 放弃回滚，恢复回滚前版本
        env.disk.rename(prev_dir, MAIN)
        vi.Version = old_version
        vi.VersionPrevious = old_version
        vi.VersionStatus = "applied"
        vi.RollbackPrevious = ""
        vi.RollbackTarget = ""
        return "aborted rollback, restored to %s" % old_version
    return "CRASH RECOVERY FAILED: neither main, target, nor backup exists"


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
    env.host_idle = False
    need_dl, nv = cmd_check_update(env)
    # 显式回滚是用户发起的动作，独立于 check 的"无动作可做"判定，必须先于 host_idle 返回
    # （F4 状态下 rollback 到其它目标版本仍然合法，只有 target == Version 才被拒绝）
    if use_rollback:
        return cmd_rollback(env, use_rollback)
    if env.host_idle:
        return "idle: no actionable update"  # F4
    if need_dl or always_download:
        cmd_download_update(env)
    return cmd_apply_update(env)
