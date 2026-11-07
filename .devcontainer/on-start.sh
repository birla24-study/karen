#!/bin/bash
set -euo pipefail

KAREN_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$KAREN_DIR"

echo "==> [Codespaces] Waiting for Docker daemon to become ready..."
until docker info > /dev/null 2>&1; do
  sleep 1
done

clean_str() {
  local val="${1:-}"
  if [[ "$val" == "-" || "$val" == "none" || "$val" == "null" ]]; then
    echo ""
  else
    echo "$val"
  fi
}

clean_num() {
  local val="${1:-}"
  local default="${2:-0}"
  if [[ "$val" =~ ^-?[0-9]+$ ]]; then
    echo "$val"
  else
    echo "$default"
  fi
}

if [[ -d .env ]]; then
  echo "==> [Codespaces] Detected .env is an invalid directory; removing..."
  rm -rf .env
fi

if [[ -d bot/config.yaml ]]; then
  echo "==> [Codespaces] Detected bot/config.yaml is an invalid directory; removing..."
  rm -rf bot/config.yaml
fi

if [[ ! -f .env ]]; then
  # Auto-generate .env from Codespaces secrets if injected as environment variables
  if [[ -n "${TELEGRAM_BOT_TOKEN:-}" || -n "${BOT_TOKEN:-}" ]]; then
    echo "==> [Codespaces] Detected Codespaces secrets in environment; generating .env..."
    BOT_TOKEN_VAL="$(clean_str "${TELEGRAM_BOT_TOKEN:-${BOT_TOKEN:-}}")"
    API_ID_VAL="$(clean_num "${TELEGRAM_API_ID:-}" 0)"
    API_HASH_VAL="$(clean_str "${TELEGRAM_API_HASH:-}")"
    ALLOWED_CHATS_VAL="$(clean_str "${TELEGRAM_ALLOWED_CHAT_IDS:-}")"
    ADMIN_IDS_VAL="$(clean_str "${ADMIN_IDS:-}")"
    MEDIA_TOKEN_VAL="$(clean_str "${MEDIA_USER_TOKEN:-}")"
    HELPER_TOKENS_VAL="$(clean_str "${HELPER_BOT_TOKENS:-}")"
    DUMP_CHANNEL_VAL="$(clean_num "${DUMP_CHANNEL_ID:-}" 0)"
    GOFILE_TOKEN_VAL="$(clean_str "${GOFILE_TOKEN:-}")"
    DB_URL_VAL="$(clean_str "${DATABASE_URL:-}")"
    FLUSH_THRESH_VAL="$(clean_num "${RIP_FLUSH_THRESHOLD_GB:-}" 20)"
    DL_MAX_VAL="$(clean_num "${TELEGRAM_DOWNLOAD_MAX_GB:-}" 5)"

    cat <<EOF > .env
TELEGRAM_BOT_TOKEN="${BOT_TOKEN_VAL}"
TELEGRAM_API_ID=${API_ID_VAL}
TELEGRAM_API_HASH="${API_HASH_VAL}"
TELEGRAM_ALLOWED_CHAT_IDS="${ALLOWED_CHATS_VAL}"
ADMIN_IDS="${ADMIN_IDS_VAL}"
MEDIA_USER_TOKEN="${MEDIA_TOKEN_VAL}"
HELPER_BOT_TOKENS="${HELPER_TOKENS_VAL}"
DUMP_CHANNEL_ID=${DUMP_CHANNEL_VAL}
GOFILE_TOKEN="${GOFILE_TOKEN_VAL}"
DATABASE_URL="${DB_URL_VAL}"
RIP_FLUSH_THRESHOLD_GB=${FLUSH_THRESH_VAL}
TELEGRAM_DOWNLOAD_MAX_GB=${DL_MAX_VAL}
EOF

    # Dynamically append all APPLE_ID_N / APPLE_PASS_N pairs present in the environment
    idx=1
    while :; do
      id_var="APPLE_ID_${idx}"
      pass_var="APPLE_PASS_${idx}"
      val_id="$(clean_str "${!id_var:-}")"
      val_pass="$(clean_str "${!pass_var:-}")"
      if [[ -n "$val_id" ]]; then
        echo "${id_var}=\"${val_id}\"" >> .env
        echo "${pass_var}=\"${val_pass}\"" >> .env
        idx=$((idx + 1))
      else
        break
      fi
    done

    echo "==> [Codespaces] .env created from Codespaces secrets ($((idx - 1)) Apple account(s))."
  else
    echo "==> [Codespaces] WARNING: .env not found and no Codespaces secrets detected."
    echo "==> Please copy .env.example to .env, fill in secrets, and run ./setup.sh once."
    exit 0
  fi
fi

# Check if initial account bootstrap has ever run
if [[ ! -f .logins/v3-accounts.txt ]]; then
  echo "==> [Codespaces] First-time setup detected. Apple login/2FA may be required."
  echo "==> Run './setup.sh' in the Codespace terminal to complete 2FA login."
fi

echo "==> [Codespaces] Generating config files..."
./generate.sh

echo "==> [Codespaces] Starting Karen stack..."
docker compose --env-file /dev/null up -d

echo "==> [Codespaces] Karen bot is running in the background."
echo "==> Tail logs anytime with: docker compose logs -f bot"

# Notify the primary admin on Telegram that Karen is awake and ready
load_env_val() {
  local key="$1"
  grep -E "^${key}=" .env 2>/dev/null | head -n 1 | cut -d'=' -f2- | tr -d '"'\'' ' || true
}
TG_TOKEN="$(load_env_val TELEGRAM_BOT_TOKEN)"
ADMINS="$(load_env_val ADMIN_IDS)"
FIRST_ADMIN="$(echo "$ADMINS" | cut -d',' -f1)"

if [[ -n "$TG_TOKEN" && -n "$FIRST_ADMIN" ]]; then
  curl -s -X POST "https://api.telegram.org/bot${TG_TOKEN}/sendMessage" \
    -d "chat_id=${FIRST_ADMIN}" \
    -d "text=🟢 Karen is online and ready for downloads!" > /dev/null || true
fi


