#!/usr/bin/env bash
# 提交并推送（使用 Windows 版 git，避免 WSL gnutls 走代理推送失败的问题）
# 用法: ./scripts/push.sh "commit message"
set -e
cd "$(dirname "$0")/.."

MSG="${1:-update}"
GIT_WIN="/mnt/d/Program Files/Git/cmd/git.exe"
GIT_CMD="git"
if [ -x "$GIT_WIN" ]; then
  GIT_CMD="$GIT_WIN"
fi

$GIT_CMD add -A
$GIT_CMD commit -m "$MSG"
$GIT_CMD push origin main
echo "✅ 已提交并推送到 origin/main"
