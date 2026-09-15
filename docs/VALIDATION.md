# Validation plan and reproducible evidence
VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914

Tests are executable evidence, not a claim that untested real-world records are accurate.
The Actions run for a commit is the controlling automated pass/fail record.

| Area | Coverage |
|---|---|
| Rust unit tests | Unknown vs zero, unit/voucher minimum, approval/subsidy boundaries, explicit excess, cash flow, zero-rate loans, scenario isolation, money bounds, deterministic results |
| Go tests | Property validation, NaN rejection, exclusive utilities, dated request validation, specificity/conflicts, strict JSON, IDs, URL policy, rate limits, authenticated service calls |
| Python tests | Temporal/geographic comparable filters, duplicates, demo isolation, no invented sparse estimate, HUD schema/year checks, money conversion, fixed origins, source transcription checks |
| Database integration | Real PostGIS, nonprivileged role, default-deny RLS, cross-tenant reads/writes, catalog grants, append-only sources, temporal acceptance/split and overlap exclusion |
| API integration | Full four-service calculation, replay hash, idempotency conflicts, unknown inputs, historical knowledge, official pilot values, tenant isolation, viewer rights, durable batch jobs, atomic bad CSV, provider-off behavior |
| Browser | Chromium login, demo property, actual analysis, replay, 1440px desktop/390px mobile screenshots, overflow assertion, logout and JavaScript error check |
| Integrity | Modified, missing, added files, owner-label changes and symlinks rejected |

## Commands
Run unit commands in README. With a disposable seeded Compose stack:

```sh
python3 tests/integration.py
docker compose run --rm -T -v "$PWD/tests:/tests:ro" admin python /tests/database_checks.py
(cd tests && npm ci && npx playwright install --with-deps chromium)
node tests/browser.mjs
```

The database and HTTP suites create only fictional test accounts/properties. Use a
separate local/CI database, not a live landlord workspace. Runtime secrets are read
from the generated `.env`, never committed or printed by these tests.

No claim is made of independent penetration testing, nationwide rent-model accuracy,
legal compliance, production availability or performance at unspecified scale.
`benchmarks/run.py` records workload count, elapsed time, failures and environment
without invented throughput figures. Benchmark outputs belong in `artifacts/`, not
in the official source catalog.
