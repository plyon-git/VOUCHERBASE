# Operations and extension guide
VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914

## Service lifecycle
`sh scripts/start.sh` creates credentials without overwriting existing `.env`, builds
images, starts PostgreSQL, applies checksum-tracked migrations, provisions database
roles and seeds the reviewed pilot plus optional synthetic demo. The API waits for
bootstrap success. Geocoding remains disabled by default. Service health is available
at `/healthz`; ordinary catalog/property errors never publish SQL or credentials.

Repeated bootstrap is idempotent for unchanged sources and demo IDs. Changed source
bytes require a new external key; changing an already-applied SQL migration fails.
Create new migration files instead. A failed migration transaction rolls back.

Changing a database password in `.env` needs matching database role administration
and a controlled restart. Simply changing the database container's environment does
not rotate an existing Postgres volume's administrator password. Back up `.env`
securely and do not commit it.

## Data updates
Use `pipeline ... import` to stage bundles; `pipeline ... approve` to publish a reviewed
successor. The API can read change events in the Evidence view. No autonomous scraper
accepts changing legal/policy tables. Schedule the CLI with a controlled external
scheduler if ongoing acquisition is required; human acceptance remains explicit.

For a future HUD revision, request the exact fiscal year and entity, verify the schema
and effective dates, then review the output before acceptance. Provider tokens stay in
environment configuration. Proprietary comparable feeds require a licensed adapter;
the included JSON import is a usable integration contract, not a scraping bypass.

## Accounts / audit
Use `admin ... tenant --name` and `--role viewer` for access. The exact name maps to a
tenant UUID during administration. Query `api_keys` through an administrative session
to identify a key UUID for revocation; never expose this table through a tenant API.
Audit includes property, import, analysis and batch creation. Replay checks the saved
numeric request, not a new geocode or fresh comparable model.

## Capacity and persistence
Property imports and jobs accept at most 1,000 items. HTTP inputs are limited to 2 MB;
Go limits service responses to 8 MB; Python caps candidate lists at 2,000; Rust allows
32 scenarios. Portfolio/list screens have documented bounded result sets (1,000
properties, 200 analyses, 100 jobs/sources). This release is a scoped landlord/pilot
workbench, not an unlimited institutional data warehouse. Add pagination and query
plans before lifting these limits.

If a worker dies, its two-minute lease expires and another worker can retry. After
three failed attempts an item is marked failed. Stable per-item idempotency prevents
an already-saved calculation from being inserted twice. Inspect job item errors and
fix the underlying input/provider issue before submitting a new job.

## Backups and production
Use an operator-managed PostgreSQL backup and test restoration into a separate volume.
Exported analyses are useful evidence but are not a database backup. Add TLS, centralized
identity, secret rotation, distributed throttling, metrics/alerting, dependency auditing,
validated PHA coverage and a documented review process before internet deployment.
No public website is deployed by pushing this repository.

## Architecture references
- https://docs.docker.com/compose/how-tos/startup-order/
- https://hub.docker.com/r/postgis/postgis
- https://www.postgresql.org/docs/current/ddl-rowsecurity.html
- https://postgis.net/docs/ST_Covers.html
- https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool
