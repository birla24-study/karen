#!/bin/bash
set -euo pipefail

KAREN_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$KAREN_DIR"

echo "==> [Codespaces] Waiting for Docker daemon to become ready..."
until docker info > /dev/null 2>&1; do
  sleep 1
done

if [[ ! -f .env ]]; then
  # Auto-generate .env from Codespaces secrets if injected as environment variables
  if [[ -n "${TELEGRAM_BOT_TOKEN:-}" || -n "${BOT_TOKEN:-}" ]]; then
    echo "==> [Codespaces] Detected Codespaces secrets in environment; generating .env..."
    BOT_TOKEN_VAL="${TELEGRAM_BOT_TOKEN:-${BOT_TOKEN:-}}"
    cat <<EOF > .env
TELEGRAM_BOT_TOKEN="${BOT_TOKEN_VAL}"
TELEGRAM_API_ID=${TELEGRAM_API_ID:-0}
TELEGRAM_API_HASH="${TELEGRAM_API_HASH:-}"
TELEGRAM_ALLOWED_CHAT_IDS="${TELEGRAM_ALLOWED_CHAT_IDS:-}"
ADMIN_IDS="${ADMIN_IDS:-}"
MEDIA_USER_TOKEN="${MEDIA_USER_TOKEN:-}"
HELPER_BOT_TOKENS="${HELPER_BOT_TOKENS:-}"
DUMP_CHANNEL_ID=${DUMP_CHANNEL_ID:-0}
GOFILE_TOKEN="${GOFILE_TOKEN:-}"
DATABASE_URL="${DATABASE_URL:-}"
RIP_FLUSH_THRESHOLD_GB=${RIP_FLUSH_THRESHOLD_GB:-20}
APPLE_ID_1="${APPLE_ID_1:-}"
APPLE_PASS_1="${APPLE_PASS_1:-}"
APPLE_ID_2="${APPLE_ID_2:-}"
APPLE_PASS_2="${APPLE_PASS_2:-}"
EOF
    echo "==> [Codespaces] .env created from Codespaces secrets."
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

