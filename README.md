# VOUCHERBASE
### Address-based voucher rental intelligence and landlord underwriting
**Parrish Lyon** · `PL-VOUCHERBASE-20260914` · Go / Rust / PostgreSQL + PostGIS / Python

A runnable four-language application, not four disconnected samples. Enter property
specifications, select and confirm an administering authority, identify tenant-paid
utilities, and compare a dated payment-standard planning reference with comparable
observations and cash-flow scenarios. Every analysis retains its source rows,
input assumptions, calculation version and replay hash.

> **A payment standard is not an approved contract rent, a universal rent ceiling,
> or a guaranteed owner payment.** VOUCHERBASE does not calculate household subsidy
> shares, approve tenants, perform official rent-reasonableness determinations,
> inspect units, or submit government forms.

## Start locally
Install Docker with Docker Compose and Python 3. No local Go or Rust install is
needed for the containerized application. On a Mac, start Docker Desktop first.

```sh
git clone https://github.com/plyon-git/VOUCHERBASE.git
cd VOUCHERBASE
sh scripts/start.sh
```

Open **http://localhost:8080**. Open your local `.env` file and use its
`VB_API_TOKEN` value in the sign-in form. This is a generated secret, not a password
published in this repository. The browser keeps it in memory, not localStorage.

Click **Load demonstration property**, then **Save & analyze**. All fixtures are
explicitly synthetic. Explore underwriting scenarios, source evidence, replay,
portfolio batch processing, CSV property import and JSON comparable import.
The interface includes downloadable import templates.

```sh
docker compose ps
docker compose logs -f api bootstrap
docker compose down        # stop, KEEP the database
```

The PostGIS image uses `linux/amd64`. Apple Silicon uses Docker's emulation for
this service. The Go, Rust and Python images build for the host platform. Do not
use `docker compose down -v` unless intentionally deleting your local database.

## Implemented workflows

| Layer | Actual responsibility |
|---|---|
| **Go** | Authenticated REST API, tenant-scoped portfolios, strict input validation, bounded CSV/JSON imports, durable leased batch jobs, timeouts, idempotency and audit events |
| **Rust** | Authoritative rent-reference and amortizing cash-flow calculations, integer-cent money, checked intermediates, deterministic scenario evaluation and saved-input replay |
| **SQL** | PostGIS address/candidate queries, effective-time + knowledge-time schedule history, overlap exclusion constraints, append-only evidence and analyses, FORCE row-level security |
| **Python** | Reviewed schedule ingestion and approval, source-change records, HUD API normalization, opt-in Census geocoding, transparent comparable selection and time-ordered property-holdout evaluation |
| **Browser** | Five-view responsive landlord workspace, property form, utility choices, financing assumptions, comparison tables, evidence inspection, export and print |

### Current coverage
A visually transcribed **Denver Housing Authority 2026** payment-standard and
utility schedule is included with its official source URL. It covers the published
0–6 bedroom categories and three utility structure categories. The hash identifies
the reviewed numeric transcription, **not the original PDF bytes**. No verified DHA
service-area polygon is supplied; the operator must confirm the administering PHA.

Separate fictional schedules and comparable observations support a credential-free
demo. They never become official figures or live-market validation. **No nationwide
PHA coverage, licensed rental listing feed or trained nationwide rent model is
claimed.** Unsupported data remains unavailable rather than being guessed.

## Enable live address lookup
Set `VB_LIVE_PROVIDERS=1` in `.env`, then run:

```sh
docker compose up -d --force-recreate python
```

The UI requests confirmation before sending an address to the Census Geocoder.
A returned coordinate is labeled an **interpolated address-range match**, not a
verified parcel. Multiple or missing matches require review. Provider failures do
not invent coordinates. Payment schedules are looked up from the accepted catalog,
not scraped afresh for every property request.

## Import reviewed payment / utility schedules
Copy a bundle following `data/denver_2026.json` into `imports/`. Keep its source URL,
effective interval, structure, utility responsibilities and integer-cent values.
Use a new `external_key` for changed source content. Acquire only permitted data.

```sh
# Stage without making it usable in calculations:
docker compose run --rm pipeline python -m voucherbase.cli import /imports/local-schedule.json

# Create an immutable accepted successor with an explicit reviewer:
docker compose run --rm pipeline python -m voucherbase.cli approve   --external-key YOUR-SOURCE-KEY --reviewer "Your Name"
```

Overlapping accepted facts are rejected unless the reviewer explicitly supplies
`--supersede`. Supersession closes the old knowledge interval and preserves any
unreplaced effective-date slices. Old saved analyses retain their inputs and hashes.
No silent last-write-wins policy is used.

## HUD benchmarks
Add `HUD_API_TOKEN` to `.env` (obtain it through HUD's documented registration).
Use an actual HUD entity identifier and explicitly reviewed effective dates:

```sh
docker compose run --rm pipeline python -m voucherbase.cli hud   --entity HUD_ENTITY_ID --year 2026 --valid-from 2025-10-01   --valid-until 2026-10-01 --output /imports/hud-benchmark.json

docker compose run --rm pipeline python -m voucherbase.cli import   /imports/hud-benchmark.json --accept --reviewer "Your Name"
```

The command saves a reviewable API snapshot, refuses year/schema mismatches, and
stores FMR/SAFMR as **benchmarks**, never as PHA payment standards. Verify any revised
publication's applicability dates instead of assuming the example dates apply.
Use the imported entity ID in the property form's HUD benchmark field.

## Accounts
The local bootstrap creates one owner account. Provision additional owner or viewer
keys, or revoke a key, using the separate administrator service:

```sh
docker compose run --rm admin python -m voucherbase.cli tenant --name "Another landlord"
docker compose run --rm admin python -m voucherbase.cli tenant --name "Local landlord" --role viewer
docker compose run --rm admin python -m voucherbase.cli revoke --key-id KEY_UUID
```

A name identifies a tenant during administrative provisioning. Reusing the exact
name adds a key to that tenant. Keys are shown once, hashed in PostgreSQL, and never
needed by the Rust engine or Python statistical model. This is API-key authentication,
not a hosted identity provider, SSO, billing or a self-service signup system.

## Verification and development

```sh
python3 scripts/integrity.py
PYTHONPATH=python python3 -m unittest discover -s python/tests -v
python3 -m unittest discover -s tests -v
node --test tests/ui-utils.test.mjs
(cd apps/api-go && go test -race ./... && go vet ./...)
cargo test --workspace --locked
```

CI also builds all images, runs the real PostgreSQL/PostGIS + Go + Rust + Python
integration suite, checks RLS with a non-superuser connection, and drives Chromium
through login, demo analysis, replay and a mobile viewport. See the **Actions** tab
for a particular commit's results; checked-in source is not itself evidence of a pass.

[Architecture](docs/ARCHITECTURE.md) · [Data and rules](docs/DATA_AND_RULES.md) ·
[Security](docs/SECURITY.md) · [Operations](docs/OPERATIONS.md) ·
[Validation](docs/VALIDATION.md) · [API](contracts/openapi.yaml)

## Limits before real-world deployment
Review the official source and local PHA rules for each lease date. This version does
not implement exception payment standards, reasonable-accommodation utility overrides,
portability decisions, household-income calculations, inspection status or special
program rules. It does not make rent approval, legal compliance, discrimination,
fraud, investment-return or tenant-eligibility determinations.

Local deployment is bound to loopback. Internet hosting requires TLS, an appropriate
identity solution, backup/restore procedures, dependency review, independent security
and policy validation, monitoring and an operator responsible for data coverage.
See NOTICE for first-party ownership and third-party attribution boundaries.
