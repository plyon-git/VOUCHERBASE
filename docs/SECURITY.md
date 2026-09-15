# Security and operating boundaries
VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914

## Implemented controls
- Random local bootstrap credentials; 0600 `.env`; only SHA-256 hashes of high-entropy
  landlord keys in the database; separate internal token and database roles.
- Owner/viewer enforcement; tenant derived from authentication, not supplied by client;
  FORCE RLS plus composite tenant/record foreign keys; scoped prepared SQL.
- Shared sources read-only to the app. Analyses, source snapshots and audit events are
  append-only. The normal application cannot obtain plaintext keys or update the catalog.
- Strict JSON, request-size limits, rate limiting, explicit timeouts and bounded batch
  sizes. Fixed upstream origins with redirects rejected. HTTP source URLs are never
  fetched by the API's source-inspection route.
- CSP, no framing, no sniffing, no third-party browser assets, no cross-origin API access,
  textContent-based rendering. Credential storage is memory-only; API responses are no-store.
- Only loopback API port published. App/Rust/Python runtime containers are non-root and
  read-only with dropped capabilities. No telemetry, destructive controls or remote kill switch.

## Threat model and limits
RLS defends against cross-tenant application mistakes, not a compromised database
administrator, a stolen application database credential capable of changing session
context, or a fully compromised trusted API. The narrowly scoped SECURITY DEFINER
functions are deliberate authentication/worker exceptions and must be reviewed with
role changes. Keep database credentials away from untrusted clients.

Internal HTTP is not appropriate across an untrusted network. API keys require a
trusted TLS reverse proxy for internet use; add SSO/MFA, centralized revocation and
appropriate secret management for a hosted product. Per-process rate limiting is not
a distributed abuse-defense service. PostgreSQL, the container runtime and dependency
updates remain operator responsibilities. Third-party providers receive explicitly
submitted addresses only when enabled; provider logs/retention are outside this app.

No real tenant identifiers or income records are required. Avoid uploading sensitive
household records. Application owner keys can export their tenant's analyses; exported
files need suitable access controls. This prototype has no automated retention/deletion
policy, billing, malware scanning, legal compliance certification or penetration test.

## Watermark
`WATERMARK.json` binds the published source files to SHA-256 digests and Parrish Lyon
attribution. It detects changes relative to a trusted baseline, not malicious changes
to both code and manifest. Retain an independently trusted commit/hash. CODEOWNERS
alone does not require review: branch protections must be configured separately.
Never run `scripts/integrity.py --write` as a way to silence an unexpected failure.

Use the repository owner's established private channel for security reports. Do not
post credentials, landlord data, or exploit-bearing records in public issues.
