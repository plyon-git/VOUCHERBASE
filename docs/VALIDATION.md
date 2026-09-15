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
| Browser | Chromium login, demo property, actual analysis, replay, 1440px desktop/390px mobile screenshots, overflow assertion, all five views, desktop/mobile sign-in and sign-out, no browser-storage credentials and JavaScript error check |
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

## Initial integration and subsequent regression checks
The initial source publication passed 34 Python pipeline/model/provider tests,
6 integrity tests, 6 browser-utility tests, 16 Go tests with the race detector,
18 Rust tests, 15 four-service API tests and 10 real PostgreSQL/PostGIS tests.
Its browser run exposed a mobile CSS defect: the account panel hid Sign out.
The narrow-screen account controls are now retained and the browser test checks
sign-out after both desktop and mobile sign-in. No hidden or forced clicks are used.

The test-only Playwright dependency is pinned to 1.63.0 with its npm-generated
lockfile. CI runs npm audit with a high-severity threshold. This does not replace
a complete dependency or application security audit. The regular read-only
workflow verifies the committed watermark without regenerating it, builds the
actual Compose services, and packages source only after all checks succeed.
A measured 200-iteration Rust CLI benchmark includes process startup overhead;
it is not a claim about bulk API throughput. The report is retained with the run.

See the VOUCHERBASE checks run on the delivered commit for the final result.
