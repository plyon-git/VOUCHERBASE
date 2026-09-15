-- VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
-- $1 authority, $2 kind, $3 bedrooms, $4 effective date, $5 knowledge timestamp.
-- ZIP/structure specificity must be resolved without blending schedules.
SELECT r.*,s.title,s.url,s.snapshot_sha256,s.retrieved_at,s.category
FROM schedule_rows r JOIN catalog_sources s ON s.id=r.source_id
WHERE authority_id=$1 AND kind=$2 AND bedrooms=$3 AND status='accepted'
  AND valid_during @> $4::date AND known_during @> $5::timestamptz
ORDER BY geo_key,structure,utility_code;
