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

if [ -d "$prefix" ]; then
    echo "卸载: $prefix"
    rm -rf "$prefix"
else
    echo "未找到安装目录: $prefix"
fi

if [ -f "$HOME/.local/bin/aly-publish" ]; then
    rm -f "$HOME/.local/bin/aly-publish"
    echo "已移除启动器: $HOME/.local/bin/aly-publish"
fi

echo "卸载完成。"
