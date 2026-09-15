-- VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
-- A boundary match suggests a candidate. It never proves the administering PHA.
SELECT id,name,boundary_verified,is_demo FROM authorities
WHERE ST_Covers(boundary,ST_SetSRID(ST_MakePoint($1,$2),4326));
-- Comparable distance uses geography/metres, not degrees.
SELECT id,ST_Distance(location::geography,ST_SetSRID(ST_MakePoint($1,$2),4326)::geography) AS metres
FROM comparables WHERE ST_DWithin(location::geography,ST_SetSRID(ST_MakePoint($1,$2),4326)::geography,3000);
