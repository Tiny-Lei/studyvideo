#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

# 逐行解析 .env（不执行 shell 展开，兼容含 & ( ) 等字符的值）
if [ -f .env ]; then
  while IFS= read -r line || [ -n "$line" ]; do
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [ -z "$line" ] && continue
    case "$line" in \#*) continue ;; esac
    case "$line" in *=*) ;; *) continue ;; esac
    key="${line%%=*}"
    value="${line#*=}"
    key="$(printf '%s' "$key" | tr -d '[:space:]')"
    case "$value" in
      \"*\") value="${value#\"}" ; value="${value%\"}" ;;
      \'*\') value="${value#\'}" ; value="${value%\'}" ;;
    esac
    export "$key=$value"
  done < .env
fi

if [ ! -x ./bin/studyvideo ]; then
  echo "未找到 ./bin/studyvideo，请先执行：make build" >&2
  exit 1
fi

exec ./bin/studyvideo "$@"
