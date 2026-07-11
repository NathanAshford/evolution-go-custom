#!/usr/bin/env bash
# =============================================================================
#  Evolution GO Custom — instalador 1-clique para Ubuntu Server (VPS)
# -----------------------------------------------------------------------------
#  100% autonomo: rode um comando e ja saia usando. Sem Coolify, sem painel.
#
#    1. Cria swap automaticamente se a RAM for baixa (evita travar o build)
#    2. Instala Docker Engine + Compose se faltarem
#    3. Clona (ou atualiza) este repositorio em /opt/evolution-go
#    4. Gera segredos fortes (GLOBAL_API_KEY + senha do banco) na 1a execucao
#    5. Compila a imagem a partir do codigo-fonte e sobe a stack
#    6. Abre a porta da app (e o SSH) no firewall, se houver um ativo
#    7. Espera ficar saudavel e imprime a URL do Manager + sua API key
#
#  Uso (recomendado):
#    curl -fsSL https://raw.githubusercontent.com/NathanAshford/evolution-go-custom/main/install.sh | sudo bash
#
#  Variaveis opcionais (env):
#    APP_PORT=4000        Porta da API/Manager                     (padrao 8080)
#    INSTALL_DIR=/opt/... Onde instalar                            (padrao /opt/evolution-go)
#    REPO_URL=...         Repositorio git a implantar
#    EVO_BRANCH=main      Branch                                   (padrao main)
#    SETUP_UFW=1          Habilitar o UFW se estiver inativo       (padrao 0)
#    CREATE_SWAP=0        Nao criar swap automaticamente           (padrao: cria se precisar)
#    SWAP_SIZE=2G         Tamanho do swap criado                   (padrao 2G)
#
#  Re-executar e seguro: atualiza o codigo e preserva seus segredos.
# =============================================================================
set -Eeuo pipefail

# ---- Configuracao -----------------------------------------------------------
REPO_URL="${REPO_URL:-https://github.com/NathanAshford/evolution-go-custom.git}"
EVO_BRANCH="${EVO_BRANCH:-main}"
INSTALL_DIR="${INSTALL_DIR:-/opt/evolution-go}"
APP_PORT="${APP_PORT:-8080}"
SETUP_UFW="${SETUP_UFW:-0}"
CREATE_SWAP="${CREATE_SWAP:-auto}"
SWAP_SIZE="${SWAP_SIZE:-2G}"
SWAP_MIN_MB="${SWAP_MIN_MB:-1900}"

# ---- Saida bonita -----------------------------------------------------------
C_RESET=$'\033[0m'; C_BOLD=$'\033[1m'; C_GREEN=$'\033[32m'; C_YELLOW=$'\033[33m'; C_RED=$'\033[31m'; C_CYAN=$'\033[36m'
say()  { printf '%s\n' "${C_CYAN}▸${C_RESET} $*"; }
ok()   { printf '%s\n' "${C_GREEN}✓${C_RESET} $*"; }
warn() { printf '%s\n' "${C_YELLOW}!${C_RESET} $*"; }
die()  { printf '%s\n' "${C_RED}✗ $*${C_RESET}" >&2; exit 1; }
banner() {
  printf '\n%s\n' "${C_BOLD}${C_GREEN}  ╔══════════════════════════════════════════════════════╗${C_RESET}"
  printf '%s\n'   "${C_BOLD}${C_GREEN}  ║        Evolution GO Custom — instalador 1-clique     ║${C_RESET}"
  printf '%s\n\n' "${C_BOLD}${C_GREEN}  ╚══════════════════════════════════════════════════════╝${C_RESET}"
}

trap 'die "Falha na instalacao na linha $LINENO. Veja a saida acima."' ERR

banner

# ---- 1. Precisa ser root ----------------------------------------------------
if [ "$(id -u)" -ne 0 ]; then
  die "Rode como root. Ex.:  curl -fsSL <url>/install.sh | sudo bash"
fi

# ---- 2. Checagem de SO ------------------------------------------------------
if [ -r /etc/os-release ]; then
  . /etc/os-release
  case "${ID:-}${ID_LIKE:-}" in
    *ubuntu*|*debian*) ok "Sistema detectado: ${PRETTY_NAME:-Linux}" ;;
    *) warn "Este instalador foca em Ubuntu/Debian; '${PRETTY_NAME:-desconhecido}' pode funcionar, mas nao foi testado." ;;
  esac
else
  warn "Nao consegui detectar o SO; assumindo Debian/Ubuntu."
fi

export DEBIAN_FRONTEND=noninteractive

# ---- 3. Swap automatico (VPS pequena) --------------------------------------
# O primeiro build compila Go + o manager React e precisa de ~2 GB. Em VPS de
# 1 GB isso trava por falta de memoria; criamos swap para deixar 100% autonomo.
ensure_swap() {
  [ "$CREATE_SWAP" = "0" ] && return 0
  local mem_kb swap_kb mem_mb
  mem_kb="$(awk '/MemTotal/{print $2}' /proc/meminfo 2>/dev/null || echo 0)"
  swap_kb="$(awk '/SwapTotal/{print $2}' /proc/meminfo 2>/dev/null || echo 0)"
  mem_mb=$(( mem_kb / 1024 ))
  # RAM suficiente, ou ja ha >=1 GB de swap: nao mexe.
  if [ "$mem_mb" -ge "$SWAP_MIN_MB" ] || [ "$swap_kb" -ge 1048576 ]; then
    return 0
  fi
  if [ -e /swapfile ]; then
    warn "RAM baixa (${mem_mb} MB), mas /swapfile ja existe — mantendo."
    return 0
  fi
  say "RAM baixa (${mem_mb} MB) — criando ${SWAP_SIZE} de swap para o build nao travar..."
  # Best-effort: qualquer falha aqui apenas avisa; nunca aborta a instalacao.
  if ! ( fallocate -l "$SWAP_SIZE" /swapfile 2>/dev/null \
         || dd if=/dev/zero of=/swapfile bs=1M count=2048 status=none 2>/dev/null ); then
    warn "Nao consegui alocar o swapfile; seguindo sem swap."
    rm -f /swapfile 2>/dev/null || true
    return 0
  fi
  chmod 600 /swapfile 2>/dev/null || true
  mkswap /swapfile >/dev/null 2>&1 || true
  if swapon /swapfile 2>/dev/null; then
    grep -q '^/swapfile ' /etc/fstab 2>/dev/null || echo '/swapfile none swap sw 0 0' >> /etc/fstab
    ok "Swap de ${SWAP_SIZE} ativado."
  else
    warn "Swapfile criado, mas nao consegui ativar (swapon) — seguindo mesmo assim."
    rm -f /swapfile 2>/dev/null || true
  fi
}
ensure_swap

# ---- 4. Pacotes base --------------------------------------------------------
say "Garantindo pacotes base (curl, git, ca-certificates, openssl)..."
if command -v apt-get >/dev/null 2>&1; then
  apt-get update -qq
  apt-get install -y -qq curl git ca-certificates openssl >/dev/null
fi
ok "Pacotes base prontos."

# ---- 5. Docker --------------------------------------------------------------
if ! command -v docker >/dev/null 2>&1; then
  say "Instalando Docker Engine (script oficial get.docker.com)..."
  curl -fsSL https://get.docker.com | sh >/dev/null
  ok "Docker instalado."
else
  ok "Docker ja presente ($(docker --version))."
fi

systemctl enable --now docker >/dev/null 2>&1 || true
docker info >/dev/null 2>&1 || die "O daemon do Docker nao esta rodando. Inicie-o e rode de novo."

# Resolve o comando compose (plugin v2 preferido, fallback v1).
if docker compose version >/dev/null 2>&1; then
  DC="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  DC="docker-compose"
else
  say "Instalando o plugin do Docker Compose..."
  apt-get install -y -qq docker-compose-plugin >/dev/null 2>&1 || true
  docker compose version >/dev/null 2>&1 || die "Docker Compose indisponivel."
  DC="docker compose"
fi
ok "Usando compose: ${DC}"

# ---- 6. Baixar o codigo -----------------------------------------------------
# Se o instalador roda de dentro de um checkout existente, usa ele mesmo.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" 2>/dev/null && pwd || true)"
if [ -n "${SCRIPT_DIR}" ] && [ -f "${SCRIPT_DIR}/deploy/docker-compose.yml" ]; then
  INSTALL_DIR="${SCRIPT_DIR}"
  ok "Rodando de um checkout existente: ${INSTALL_DIR}"
elif [ -d "${INSTALL_DIR}/.git" ]; then
  say "Atualizando instalacao existente em ${INSTALL_DIR}..."
  git -C "${INSTALL_DIR}" fetch --depth 1 origin "${EVO_BRANCH}" >/dev/null 2>&1
  git -C "${INSTALL_DIR}" checkout -q "${EVO_BRANCH}"
  git -C "${INSTALL_DIR}" reset --hard "origin/${EVO_BRANCH}" >/dev/null 2>&1
  ok "Repositorio atualizado."
else
  say "Clonando ${REPO_URL} (branch ${EVO_BRANCH}) em ${INSTALL_DIR}..."
  mkdir -p "$(dirname "${INSTALL_DIR}")"
  git clone --depth 1 --branch "${EVO_BRANCH}" "${REPO_URL}" "${INSTALL_DIR}" >/dev/null 2>&1
  ok "Repositorio clonado."
fi

DEPLOY_DIR="${INSTALL_DIR}/deploy"
[ -f "${DEPLOY_DIR}/docker-compose.yml" ] || die "deploy/docker-compose.yml nao encontrado em ${INSTALL_DIR}."

# ---- 7. Segredos / .env -----------------------------------------------------
# Detect the public base URL early (used for PASSKEY_PUBLIC_URL and the summary).
PUBLIC_IP="$(curl -fsS https://api.ipify.org 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}' || echo 'SEU_IP')"

ENV_FILE="${DEPLOY_DIR}/.env"
gen_secret() { openssl rand -hex 24 2>/dev/null || head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'; }

if [ -f "${ENV_FILE}" ]; then
  ok "Ja existe ${ENV_FILE} — mantendo os segredos atuais."
  API_KEY="$(grep -E '^GLOBAL_API_KEY=' "${ENV_FILE}" | head -1 | cut -d= -f2-)"
else
  say "Gerando segredos fortes e aleatorios..."
  API_KEY="$(gen_secret)"
  DB_PASS="$(gen_secret)"
  umask 077
  cat > "${ENV_FILE}" <<EOF
# Gerado por install.sh em $(date -u +%Y-%m-%dT%H:%M:%SZ). Mantenha este arquivo em segredo.
SERVER_PORT=${APP_PORT}
CLIENT_NAME=evolution
OS_NAME=Linux

GLOBAL_API_KEY=${API_KEY}

POSTGRES_USER=postgres
POSTGRES_PASSWORD=${DB_PASS}
POSTGRES_AUTH_DB=postgresql://postgres:${DB_PASS}@postgres:5432/evogo_auth?sslmode=disable
POSTGRES_USERS_DB=postgresql://postgres:${DB_PASS}@postgres:5432/evogo_users?sslmode=disable
DATABASE_SAVE_MESSAGES=false

WADEBUG=INFO
LOGTYPE=console
LOG_DIRECTORY=/app/logs
LOG_MAX_SIZE=100
LOG_MAX_BACKUPS=5
LOG_MAX_AGE=30
LOG_COMPRESS=true

CONNECT_ON_STARTUP=false
WEBHOOK_FILES=true

WEBHOOK_URL=
AMQP_URL=
AMQP_GLOBAL_ENABLED=false
NATS_URL=
NATS_GLOBAL_ENABLED=false
EVENT_IGNORE_GROUP=false
EVENT_IGNORE_STATUS=true

MINIO_ENABLED=false

PROXY_HOST=
PROXY_PORT=
PROXY_USERNAME=
PROXY_PASSWORD=

QRCODE_MAX_COUNT=5
CHECK_USER_EXISTS=true

# Passkey pairing — public base URL of THIS API (used when WhatsApp requires a
# passkey to link a device). Defaults to this server's IP; change it to your
# own domain (with HTTPS) in production. See docs/wiki/guias-api/passkey-pairing.md.
PASSKEY_PUBLIC_URL=http://${PUBLIC_IP}:${APP_PORT}
EOF
  chmod 600 "${ENV_FILE}"
  ok "Segredos salvos em ${ENV_FILE} (chmod 600)."
fi

# Mantem SERVER_PORT em sincronia com APP_PORT ao re-executar com outra porta.
if grep -qE '^SERVER_PORT=' "${ENV_FILE}"; then
  sed -i -E "s/^SERVER_PORT=.*/SERVER_PORT=${APP_PORT}/" "${ENV_FILE}"
fi
APP_PORT="$(grep -E '^SERVER_PORT=' "${ENV_FILE}" | head -1 | cut -d= -f2-)"

# ---- 8. Build & subida ------------------------------------------------------
say "Compilando a imagem do codigo-fonte e subindo a stack (pode levar alguns minutos)..."
( cd "${DEPLOY_DIR}" && APP_PORT="${APP_PORT}" ${DC} up -d --build )
ok "Containers iniciados."

# ---- 9. Abrir portas no firewall (seguro, sem lockout) ---------------------
open_ports() {
  # UFW: se ativo, apenas ADICIONA regras (nunca derruba o SSH).
  if command -v ufw >/dev/null 2>&1; then
    if ufw status 2>/dev/null | grep -qi "Status: active"; then
      say "UFW ativo — liberando SSH e a porta ${APP_PORT}..."
      ufw allow OpenSSH >/dev/null 2>&1 || ufw allow 22/tcp >/dev/null 2>&1 || true
      ufw allow "${APP_PORT}/tcp" >/dev/null 2>&1 || true
      ok "Porta ${APP_PORT} liberada no UFW."
      return
    elif [ "$SETUP_UFW" = "1" ]; then
      say "Habilitando o UFW (liberando SSH e a porta ${APP_PORT} antes)..."
      ufw allow OpenSSH >/dev/null 2>&1 || ufw allow 22/tcp >/dev/null 2>&1 || true
      ufw allow "${APP_PORT}/tcp" >/dev/null 2>&1 || true
      yes | ufw enable >/dev/null 2>&1 || true
      ok "UFW habilitado com a porta ${APP_PORT} liberada."
      return
    fi
  fi
  # firewalld: se ativo, abre a porta permanentemente.
  if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
    say "firewalld ativo — abrindo a porta ${APP_PORT}..."
    firewall-cmd --permanent --add-port="${APP_PORT}/tcp" >/dev/null 2>&1 || true
    firewall-cmd --permanent --add-service=ssh >/dev/null 2>&1 || true
    firewall-cmd --reload >/dev/null 2>&1 || true
    ok "Porta ${APP_PORT} liberada no firewalld."
    return
  fi
  warn "Nenhum firewall de host ativo detectado — a porta ${APP_PORT} ja fica acessivel via Docker."
  warn "Se sua VPS tem firewall no painel do provedor (AWS/GCP/Oracle/etc.), libere a porta ${APP_PORT}/tcp la."
}
open_ports

# ---- 10. Esperar ficar saudavel --------------------------------------------
say "Aguardando a API ficar saudavel..."
healthy=0
for _ in $(seq 1 40); do
  if curl -fsS "http://127.0.0.1:${APP_PORT}/manager" >/dev/null 2>&1; then healthy=1; break; fi
  sleep 3
done
if [ "${healthy}" != "1" ]; then
  warn "A checagem de saude expirou. Ultimas linhas do log:"
  ( cd "${DEPLOY_DIR}" && ${DC} logs --tail 30 evolution-go 2>&1 | sed 's/^/    /' ) || true
fi

# ---- 11. Resumo -------------------------------------------------------------
PUBLIC_IP="${PUBLIC_IP:-$(curl -fsS https://api.ipify.org 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}' || echo 'SEU_IP')}"

printf '\n%s\n' "${C_BOLD}${C_GREEN}──────────────────────────────────────────────────────────${C_RESET}"
if [ "${healthy}" = "1" ]; then
  ok "Evolution GO Custom esta no ar!"
else
  warn "Containers subiram, mas a checagem de saude nao respondeu a tempo (pode estar terminando de iniciar)."
fi
cat <<EOF

  ${C_BOLD}Manager:${C_RESET}      http://${PUBLIC_IP}:${APP_PORT}/manager
  ${C_BOLD}API base:${C_RESET}     http://${PUBLIC_IP}:${APP_PORT}
  ${C_BOLD}Swagger:${C_RESET}      http://${PUBLIC_IP}:${APP_PORT}/swagger/index.html
  ${C_BOLD}API key:${C_RESET}      ${API_KEY}
                (envie no header HTTP  apikey: <key>)

  ${C_BOLD}Comece agora (jeito mais facil):${C_RESET}
    1) Abra o Manager acima e faca login com a API key
    2) Crie uma instancia e escaneie o QR Code no WhatsApp

  ${C_BOLD}Ou via API — cria uma instancia:${C_RESET}
    curl -X POST http://${PUBLIC_IP}:${APP_PORT}/instance/create \\
      -H "apikey: ${API_KEY}" -H "Content-Type: application/json" \\
      -d '{"name":"minha-instancia","token":"minha-instancia-token"}'

  ${C_BOLD}Pasta:${C_RESET}        ${INSTALL_DIR}
  ${C_BOLD}Segredos:${C_RESET}     ${ENV_FILE}

  ${C_BOLD}Gerenciar:${C_RESET}
    cd ${DEPLOY_DIR}
    ${DC} ps                 # status
    ${DC} logs -f            # logs
    ${DC} restart            # reiniciar
    ${DC} down               # parar (mantem os dados)
    ${DC} up -d --build      # atualizar apos 'git pull'

EOF
printf '%s\n\n' "${C_BOLD}${C_GREEN}──────────────────────────────────────────────────────────${C_RESET}"
