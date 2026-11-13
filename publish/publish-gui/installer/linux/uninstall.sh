#!/bin/sh
# AlyPublish Linux 安装包的卸载脚本：移走安装树与便捷启动器。
#
# 用法：
#   ./uninstall.sh --prefix /opt/aly/AlyPublish
set -eu

prefix="/opt/aly/AlyPublish"

while [ $# -gt 0 ]; do
    case "$1" in
        --prefix) prefix="$2"; shift 2 ;;
        --prefix=*) prefix="${1#*=}" ; shift ;;
        *) echo "未知参数: $1" >&2; exit 1 ;;
    esac
done

# 审查 publish 轻微⑦：rm -rf 防呆。
# 旧实现直接 `rm -rf "$prefix"`，`--prefix /`、`/usr`、`/home/user` 会照删；
# GNU rm 的 preserve-root 只拦字面 "/"，拦不住这些。这里要求：
#   1) 非空、非 "/"、规范化后至少两级（如 /opt/xxx）；
#   2) 目录里必须存在本安装包的标记文件（install 时写入），否则拒绝删除。
prefix=$(printf '%s' "$prefix" | sed 's:/*$::')
case "$prefix" in
    "" | "/")
        echo "拒绝卸载：--prefix 不能为空或根目录" >&2
        exit 1
        ;;
esac
depth=$(printf '%s' "$prefix" | tr -cd '/' | wc -c)
if [ "$depth" -lt 2 ]; then
    echo "拒绝卸载：--prefix 过浅（$prefix），至少要是 /opt/<name> 这一级" >&2
    exit 1
fi

if [ -d "$prefix" ]; then
    if [ ! -f "$prefix/.aly-publish-install" ]; then
        echo "拒绝卸载：$prefix 中没有本安装包标记文件 .aly-publish-install" >&2
        echo "          （请确认这是 AlyPublish 的安装目录，或用 --prefix 指定正确路径）" >&2
        exit 1
    fi
    echo "卸载: $prefix"
    rm -rf "$prefix"
else
    echo "未找到安装目录: $prefix"
fi

if [ -f "$HOME/.local/bin/aly-publish" ]; then
    rm -f "$HOME/.local/bin/aly-publish"
    echo "已移除启动器: $HOME/.local/bin/aly-publish"
fi
# 审查 publish 轻微⑧：GUI 启动器改名后也要清理（旧版本装的是 aly-publish）
if [ -f "$HOME/.local/bin/aly-publish-gui" ]; then
    rm -f "$HOME/.local/bin/aly-publish-gui"
    echo "已移除启动器: $HOME/.local/bin/aly-publish-gui"
fi

echo "卸载完成。"
