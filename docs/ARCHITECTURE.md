# Architecture
VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914

```
Browser (no stored credentials)
  -> Go HTTP API :8080
       -> PostgreSQL / PostGIS :5432
       -> Rust calculation service :8081
       -> Python data service :8082
Python administrative ingestion -> accepted catalog / source history
```

Only the Go API has a host port, bound to 127.0.0.1. HTTP between containers is for
local development; use encrypted service networking for a distributed deployment.
The internal token is separate from landlord keys. Neither numeric service has a
landlord database credential.

## Request path
1. Authenticate the SHA-256 hash of a random bearer key with a narrow database
   function. Resolve tenant and owner/viewer role from the database, never a client header.
2. Validate the address/specification and analysis request. Start a transaction and
   set the transaction-local tenant context. RLS and composite foreign keys isolate records.
3. Resolve accepted schedule facts for **both** effective date and knowledge timestamp.
   Require authority confirmation. Prefer exact ZIP/structure rows; reject equally specific conflicts.
4. Obtain utility components for the smaller unit/voucher bedroom size when entitlement
   is supplied. Unknown responsibilities stay null; explicit all-owner-paid responsibilities are zero.
5. Snapshot relevant facts and obtain property-comparable candidates with PostGIS.
   End the read transaction before network calculation calls.
6. Rust computes the only authoritative financial outputs. Python filters/ranks
   comparable observations without inventing dollar adjustments.
7. Persist immutable input/result JSON, source identifiers, engine version and hashes.
   Serialize tenant/idempotency-key writes with an advisory lock. Retrying an identical
   key returns the original result; different input returns 409.

## Batch jobs
Go runs two bounded workers. `FOR UPDATE SKIP LOCKED` claims an item under a two-minute
lease with a unique lease token. Each job item has at most three attempts, a stable
idempotency key and a 40-second processing deadline. Expired leases can be retried;
late workers cannot finalize a replacement worker's lease. Failures remain visible.
A narrow SECURITY DEFINER function lists tenant IDs with queued work; it does not
return property records or keys. Jobs use ordinary RLS transactions after tenant selection.

## Temporal catalog
`valid_during` states when a fact applies to a lease. `known_during` states when that
version was accepted into this installation. PostgreSQL exclusion constraints prevent
accepted facts with the same policy key from overlapping on both axes.

A pending import cannot affect rent analysis. Acceptance creates an immutable
reviewed successor. Explicit supersession closes the old knowledge interval and
inserts new current slices; earlier queries can reproduce what was known before.
Sources themselves are immutable. This is installation knowledge time, not a claim
that the operator knew a rule on its original government publication date.

## Tenant and input boundaries
Shared catalog facts are read-only to the application role. Tenant properties,
comparables, jobs, results and audit rows require FORCE RLS. Database-admin credentials
exist only in the local bootstrap/admin services. Review the threat model in SECURITY.

All amounts are integer cents; rates use basis points (100 = 1%). Rust uses bounded
inputs and checked i128 intermediate operations. Amortization necessarily uses a
floating exponential and rounds the payment to a cent; it is not a lender quote.
SQL JSONB reorders fields, so calculation replay compares the deterministic raw Rust
response hash, not arbitrary database JSON whitespace. Source hashes use a canonical
JSON transcription. Neither hash is a digital signature.

## Repository design
Go owns request orchestration and persistence; Rust owns financial logic; Python owns
source normalization and comparable methodology; SQL owns relational, spatial and
historical invariants. There is no duplicate Python rent calculator and no AI API dependency.
