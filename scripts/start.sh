#!/bin/sh
# VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
set -eu
cd "$(dirname "$0")/.."
command -v docker >/dev/null || { echo "Docker with Compose is required." >&2; exit 1; }
python3 scripts/configure.py
docker compose up --build -d
echo "Open http://localhost:8080. Sign in using VB_API_TOKEN from your local .env."
echo "Status: docker compose ps. Logs: docker compose logs -f api bootstrap."
