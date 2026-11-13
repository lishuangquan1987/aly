#!/bin/sh
# AlyPublish Linux 安装包的安装脚本。
#
# 压缩包（AlyPublish-linux-<rid>.zip）在 Linux 上就是“安装包”：解压后运行本脚本，
# 它把整棵树（GUI 本体 + payload/ + 运行库）装到一个前缀目录，并补齐可执行位。
# zip 不保留可执行位，所以这里必须 chmod —— 从包里单独取 CLI/更新器时同样要 chmod。
#
# 用法：
#   ./install.sh                                        # 默认装到 /opt/aly/AlyPublish（需要写权限）
#   ./install.sh --prefix $HOME/.local/lib/AlyPublish   # 无 root 时用这种
set -eu

prefix="/opt/aly/AlyPublish"

while [ $# -gt 0 ]; do
    case "$1" in
        --prefix) prefix="$2"; shift 2 ;;
        --prefix=*) prefix="${1#*=}" ; shift ;;
        *) echo "未知参数: $1" >&2; exit 1 ;;
    esac
done

here="$(cd "$(dirname "$0")" && pwd)"
if [ ! -d "$here" ]; then
    echo "包目录不存在: $here" >&2
    exit 1
fi

echo "安装 AlyPublish: $here -> $prefix"
mkdir -p "$prefix"
cp -a "$here/." "$prefix/"
# 审查 publish 轻微⑦：写安装标记，卸载脚本据此拒绝删除"非本包目录"
printf '%s\n' "AlyPublish installer marker" > "$prefix/.aly-publish-install"

# 可执行位：GUI 本体（单文件发布叫 AlyPublish）、发布 CLI、更新器与两个脚本。
chmod 0755 "$prefix/AlyPublish" 2>/dev/null || true
chmod 0755 "$prefix/publish-cli"/*/aly-publish 2>/dev/null || true
chmod 0755 "$prefix/client"/*/aly-client "$prefix/client"/*/aly-client-ui 2>/dev/null || true
chmod 0755 "$prefix/install.sh" "$prefix/uninstall.sh" 2>/dev/null || true

# 便捷启动器：$HOME/.local/bin 通常已在 PATH 上。
#
# 审查 publish 轻微⑧：旧实现把 GUI 启动器命名为 `aly-publish`——与真正的发布
# CLI 撞名，用户敲 `aly-publish status` 会弹出 GUI 窗口。这里改为：
#   · `aly-publish-gui`  → GUI
#   · `aly-publish`      → 真正的 CLI shim（按 rid 探测 publish-cli/<rid>/aly-publish）
bindir="$HOME/.local/bin"
if [ -d "$bindir" ]; then
    cat > "$bindir/aly-publish-gui" <<EOF
#!/bin/sh
exec "$prefix/AlyPublish" "\$@"
EOF
    chmod 0755 "$bindir/aly-publish-gui"

    cat > "$bindir/aly-publish" <<EOF
#!/bin/sh
# 发布 CLI 启动器：在安装树的 publish-cli/<rid>/ 下探测可执行文件。
for f in "$prefix/publish-cli"/*/aly-publish; do
    if [ -x "\$f" ]; then
        exec "\$f" "\$@"
    fi
done
echo "未找到 publish-cli（安装树: $prefix）" >&2
exit 1
EOF
    chmod 0755 "$bindir/aly-publish"
    echo "已写入启动器: $bindir/aly-publish（CLI）、$bindir/aly-publish-gui（GUI）"
fi

echo
echo "安装完成。"
echo "  启动:      $prefix/AlyPublish"
echo "  发布 CLI:  $prefix/publish-cli/<rid>/aly-publish"
echo "  更新器:    $prefix/client/<rid>/aly-client(-ui)"
echo "  卸载:      $prefix/uninstall.sh --prefix $prefix"
