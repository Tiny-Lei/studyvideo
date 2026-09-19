#!/usr/bin/env bash
#
# StudyVideo systemd 一键安装/升级脚本
#
# 用法（需要 root 或 sudo）：
#   ./deploy/install.sh                     # 安装/升级（使用源码构建好的 bin/studyvideo）
#   ./deploy/install.sh --release-url URL   # 从发布包 URL 安装
#   ./deploy/install.sh --uninstall         # 卸载（保留数据）
#
# 安装内容：
#   /opt/studyvideo/bin/studyvideo   程序
#   /opt/studyvideo/.env             配置（已存在则不覆盖）
#   /opt/studyvideo/data             PDF 资料目录
#   /etc/systemd/system/studyvideo.service
#
set -euo pipefail

APP_DIR=/opt/studyvideo
SERVICE_NAME=studyvideo
SERVICE_FILE=/etc/systemd/system/${SERVICE_NAME}.service
SERVICE_USER=studyvideo

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
RELEASE_URL=""
UNINSTALL=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --release-url) RELEASE_URL="$2"; shift 2 ;;
    --uninstall) UNINSTALL=1; shift ;;
    -h|--help) sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "未知参数: $1" >&2; exit 1 ;;
  esac
done

log()  { echo -e "\033[32m[install]\033[0m $*"; }
warn() { echo -e "\033[33m[install]\033[0m $*"; }
die()  { echo -e "\033[31m[install]\033[0m $*" >&2; exit 1; }

require_root() {
  [[ $EUID -eq 0 ]] || die "请使用 root 或 sudo 运行本脚本"
}

require_systemd() {
  command -v systemctl >/dev/null || die "未检测到 systemd，请使用 Docker 部署或手动运行"
}

uninstall() {
  require_root
  log "停止并禁用服务…"
  systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
  systemctl disable "${SERVICE_NAME}" 2>/dev/null || true
  rm -f "${SERVICE_FILE}"
  systemctl daemon-reload
  warn "已卸载服务。程序与数据保留在 ${APP_DIR}（确认无用后可手动删除）"
}

install_binary() {
  mkdir -p "${APP_DIR}/bin" "${APP_DIR}/data"

  if [[ -n "$RELEASE_URL" ]]; then
    require_cmd curl tar
    tmp="$(mktemp -d)"
    log "下载发布包 ${RELEASE_URL}"
    curl -fL "${RELEASE_URL}" -o "${tmp}/pkg.tar.gz"
    tar xzf "${tmp}/pkg.tar.gz" -C "${tmp}"
    bin="$(find "${tmp}" -maxdepth 2 -name studyvideo -type f | head -1)"
    [[ -n "$bin" ]] || die "发布包中未找到 studyvideo 可执行文件"
    install -m 0755 "$bin" "${APP_DIR}/bin/studyvideo"
    rm -rf "${tmp}"
  else
    [[ -x "${PROJECT_DIR}/bin/studyvideo" ]] || die "未找到 ${PROJECT_DIR}/bin/studyvideo，请先执行 make build，或使用 --release-url"
    install -m 0755 "${PROJECT_DIR}/bin/studyvideo" "${APP_DIR}/bin/studyvideo"
  fi
  log "程序已安装到 ${APP_DIR}/bin/studyvideo"
}

require_cmd() {
  for c in "$@"; do
    command -v "$c" >/dev/null || die "缺少命令：$c"
  done
}

install_service() {
  if [[ -f "${PROJECT_DIR}/deploy/studyvideo.service" ]]; then
    install -m 0644 "${PROJECT_DIR}/deploy/studyvideo.service" "${SERVICE_FILE}"
  else
    # 独立运行本脚本（未随仓库分发）时写入等价的内置单元文件
    cat > "${SERVICE_FILE}" <<'EOF'
[Unit]
Description=StudyVideo 题目讲解视频站
After=network-online.target
Wants=network-online.target
StartLimitIntervalSec=60
StartLimitBurst=5

[Service]
Type=simple
User=studyvideo
Group=studyvideo
WorkingDirectory=/opt/studyvideo
EnvironmentFile=/opt/studyvideo/.env
ExecStart=/opt/studyvideo/bin/studyvideo
KillSignal=SIGTERM
TimeoutStopSec=15
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=studyvideo
NoNewPrivileges=true
PrivateTmp=true
PrivateDevices=true
ProtectSystem=full
ProtectHome=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
RestrictNamespaces=true
LockPersonality=true
ReadWritePaths=/opt/studyvideo/data
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF
  fi
  log "systemd 单元已安装：${SERVICE_FILE}"
}

create_user() {
  if ! id "${SERVICE_USER}" >/dev/null 2>&1; then
    useradd --system --home-dir "${APP_DIR}" --shell /usr/sbin/nologin "${SERVICE_USER}"
    log "已创建系统用户 ${SERVICE_USER}"
  fi
  chown -R "${SERVICE_USER}:${SERVICE_USER}" "${APP_DIR}"
}

init_env() {
  if [[ -f "${APP_DIR}/.env" ]]; then
    log "配置文件已存在，保留现有 ${APP_DIR}/.env"
    return
  fi
  local admin_pass session_secret
  admin_pass="$(openssl rand -base64 12 2>/dev/null || head -c 12 /dev/urandom | base64)"
  session_secret="$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p -c 32)"
  cat > "${APP_DIR}/.env" <<EOF
HTTP_ADDR=:8080
DB_DSN=studyvideo:studyvideo123@tcp(127.0.0.1:3306)/studyvideo?parseTime=true&charset=utf8mb4&loc=Local&timeout=5s
ADMIN_PASSWORD=${admin_pass}
SESSION_SECRET=${session_secret}
TRUST_PROXY=auto
COOKIE_SECURE=false
DATA_DIR=${APP_DIR}/data
STATS_KEEP_DAYS=730
CHECK_INTERVAL_MINUTES=360
CHECK_ON_START=true
LOG_LEVEL=info
LOG_FORMAT=text
EOF
  chmod 600 "${APP_DIR}/.env"
  chown "${SERVICE_USER}:${SERVICE_USER}" "${APP_DIR}/.env"

  # 用 root 生成强密码并写入文件：需在日志里提示，否则运营看不到
  cat > "${APP_DIR}/INITIAL_ADMIN_PASSWORD.txt" <<EOF
管理后台初始密码（请尽快登录修改 ADMIN_PASSWORD 并删除本文件）：
${admin_pass}
EOF
  chmod 600 "${APP_DIR}/INITIAL_ADMIN_PASSWORD.txt"
  log "已生成 ${APP_DIR}/.env，初始管理密码见 ${APP_DIR}/INITIAL_ADMIN_PASSWORD.txt"
  warn "请修改 .env 中的 DB_DSN 指向真实数据库"
}

main() {
  require_root
  require_systemd

  if [[ $UNINSTALL -eq 1 ]]; then
    uninstall
    exit 0
  fi

  log "开始安装 StudyVideo…"
  install_binary
  create_user
  init_env
  install_service
  chown -R "${SERVICE_USER}:${SERVICE_USER}" "${APP_DIR}"
  systemctl daemon-reload
  systemctl enable --now "${SERVICE_NAME}"
  log "安装完成，服务状态："
  systemctl --no-pager --lines=0 status "${SERVICE_NAME}" || true
  echo
  log "常用命令："
  echo "  journalctl -u ${SERVICE_NAME} -f     # 查看日志"
  echo "  systemctl restart ${SERVICE_NAME}    # 重启"
  echo "  vim ${APP_DIR}/.env                  # 修改配置"
  echo "  ${APP_DIR}/bin/studyvideo --version  # 查看版本"
}

main
