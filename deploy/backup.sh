#!/usr/bin/env bash
#
# StudyVideo 自动备份脚本
#
# 备份内容：
#   1. MySQL 数据库（mysqldump，含表结构与数据）
#   2. PDF 资料目录（DATA_DIR，默认 /data/studyvideo/data）
#
# 用法：
#   ./deploy/backup.sh                       # 使用默认配置
#   APP_DIR=/data/studyvideo BACKUP_DIR=/data/backup KEEP_DAYS=30 ./deploy/backup.sh
#
# 环境变量：
#   APP_DIR     应用目录（读取其中的 .env 获取 DB_DSN） 默认 /data/studyvideo
#   BACKUP_DIR  备份输出目录                            默认 /data/backup
#   KEEP_DAYS   保留天数，超期自动清理                   默认 30
#   MYSQL_USER / MYSQL_PASSWORD / MYSQL_DATABASE        默认从 .env 的 DB_DSN 解析
#
set -euo pipefail

APP_DIR="${APP_DIR:-/data/studyvideo}"
BACKUP_DIR="${BACKUP_DIR:-/data/backup}"
KEEP_DAYS="${KEEP_DAYS:-30}"
STAMP="$(date +%F_%H%M%S)"

log() { echo "[$(date '+%F %T')] $*"; }
die() { echo "[$(date '+%F %T')] 错误: $*" >&2; exit 1; }

[[ -f "${APP_DIR}/.env" ]] || die "未找到 ${APP_DIR}/.env"
mkdir -p "${BACKUP_DIR}"

# 从 .env 解析 DB_DSN（格式 user:pass@tcp(host:port)/dbname?...）
DSN="$(grep -E '^DB_DSN=' "${APP_DIR}/.env" | head -1 | cut -d= -f2-)"
[[ -n "$DSN" ]] || die ".env 中缺少 DB_DSN"

if [[ -z "${MYSQL_USER:-}" || -z "${MYSQL_PASSWORD:-}" || -z "${MYSQL_DATABASE:-}" ]]; then
  creds="${DSN%%@tcp*}"
  MYSQL_USER="${creds%%:*}"
  MYSQL_PASSWORD="${creds#*:}"
  rest="${DSN#*)/}"
  MYSQL_DATABASE="${rest%%\?*}"
fi

# ---------- 1) 数据库备份 ----------
DB_FILE="${BACKUP_DIR}/db_${STAMP}.sql.gz"
log "备份数据库 ${MYSQL_DATABASE} -> ${DB_FILE}"
mysqldump --single-transaction --quick --no-tablespaces --routines --events \
  -u"${MYSQL_USER}" -p"${MYSQL_PASSWORD}" "${MYSQL_DATABASE}" | gzip > "${DB_FILE}.tmp"
mv "${DB_FILE}.tmp" "${DB_FILE}"

# ---------- 2) PDF 资料目录备份 ----------
DATA_DIR="$(grep -E '^DATA_DIR=' "${APP_DIR}/.env" | head -1 | cut -d= -f2- || true)"
DATA_DIR="${DATA_DIR:-${APP_DIR}/data}"
if [[ -d "${DATA_DIR}" ]]; then
  DATA_FILE="${BACKUP_DIR}/data_${STAMP}.tar.gz"
  log "备份资料目录 ${DATA_DIR} -> ${DATA_FILE}"
  tar czf "${DATA_FILE}.tmp" -C "$(dirname "${DATA_DIR}")" "$(basename "${DATA_DIR}")"
  mv "${DATA_FILE}.tmp" "${DATA_FILE}"
else
  log "资料目录不存在，跳过: ${DATA_DIR}"
fi

# ---------- 3) 清理过期备份 ----------
find "${BACKUP_DIR}" -maxdepth 1 -name 'db_*.sql.gz' -mtime "+${KEEP_DAYS}" -delete 2>/dev/null || true
find "${BACKUP_DIR}" -maxdepth 1 -name 'data_*.tar.gz' -mtime "+${KEEP_DAYS}" -delete 2>/dev/null || true

log "备份完成，当前备份文件："
ls -lh "${BACKUP_DIR}"/db_*.sql.gz "${BACKUP_DIR}"/data_*.tar.gz 2>/dev/null | tail -6
