-- VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
-- Execute within SET LOCAL app.tenant_id transaction under vb_app (not table owner).
SELECT p.id,p.address,a.id AS latest_analysis_id,a.result
FROM properties p LEFT JOIN LATERAL (
 SELECT id,result FROM analyses WHERE property_id=p.id ORDER BY created_at DESC LIMIT 1
) a ON true ORDER BY p.created_at DESC;
