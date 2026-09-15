#!/usr/bin/env bash
# VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
set -euo pipefail
cd "$(dirname "$0")/.."
mkdir -p artifacts
python3 scripts/integrity.py
PYTHONPATH=python python3 -m unittest discover -s python/tests -v
python3 -m unittest discover -s tests -v
node --test tests/ui-utils.test.mjs
node --check apps/api-go/web/app.js
(cd apps/api-go && go test -mod=readonly -race ./... && go vet -mod=readonly ./...)
cargo test --workspace --locked
cargo fmt --all -- --check
cargo build --release -p rent-core --locked
python3 benchmarks/run.py --iterations 200
python3 scripts/configure.py
docker compose up --build -d
python3 tests/integration.py
docker compose run --rm -T -v "$PWD/tests:/tests:ro" admin python /tests/database_checks.py
(cd tests && npm ci --ignore-scripts && npm audit --audit-level=high && npx playwright install --with-deps chromium)
node tests/browser.mjs
python3 scripts/integrity.py
echo "All VOUCHERBASE unit, real-database, four-service, browser and integrity checks passed."
