-- VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914
CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE TABLE tenants(id uuid PRIMARY KEY, name text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE api_keys(id uuid PRIMARY KEY, tenant_id uuid NOT NULL REFERENCES tenants(id), label text NOT NULL, token_hash text NOT NULL UNIQUE CHECK(length(token_hash)=64), role text NOT NULL CHECK(role IN ('owner','viewer')), expires_at timestamptz, revoked_at timestamptz);
CREATE FUNCTION authenticate_key(hash text) RETURNS TABLE(tenant_id uuid,label text,role text) LANGUAGE sql STABLE SECURITY DEFINER SET search_path=public,pg_temp AS $$ SELECT k.tenant_id,k.label,k.role FROM api_keys k WHERE k.token_hash=hash AND k.revoked_at IS NULL AND (k.expires_at IS NULL OR k.expires_at>now()) $$;
REVOKE ALL ON FUNCTION authenticate_key(text) FROM PUBLIC;

CREATE TABLE catalog_sources(
 id uuid PRIMARY KEY, external_key text NOT NULL UNIQUE, title text NOT NULL,
 url text NOT NULL, category text NOT NULL CHECK(category IN ('official','synthetic','user')),
 snapshot_sha256 text NOT NULL CHECK(length(snapshot_sha256)=64), document_sha256 text,
 retrieved_at timestamptz NOT NULL, reviewed_by text, reviewed_at timestamptz,
 snapshot jsonb NOT NULL, notes text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE authorities(
 id text PRIMARY KEY, name text NOT NULL, state text NOT NULL, is_demo boolean NOT NULL DEFAULT false,
 boundary geometry(MultiPolygon,4326), boundary_source text,
 boundary_verified boolean NOT NULL DEFAULT false
);
CREATE INDEX authority_boundary_idx ON authorities USING gist(boundary);
CREATE TABLE schedule_rows(
 id uuid PRIMARY KEY, authority_id text NOT NULL REFERENCES authorities(id),
 kind text NOT NULL CHECK(kind IN ('payment_standard','utility','fmr','safmr')),
 geo_key text NOT NULL DEFAULT '*', bedrooms integer NOT NULL CHECK(bedrooms BETWEEN 0 AND 6),
 structure text NOT NULL DEFAULT 'any', utility_code text NOT NULL DEFAULT 'none',
 value_cents bigint NOT NULL CHECK(value_cents BETWEEN 0 AND 1000000000000),
 valid_during daterange NOT NULL CHECK(NOT isempty(valid_during)),
 known_during tstzrange NOT NULL CHECK(NOT isempty(known_during)),
 source_id uuid NOT NULL REFERENCES catalog_sources(id),
 status text NOT NULL CHECK(status IN ('pending','accepted','rejected')),
 EXCLUDE USING gist(authority_id WITH =,kind WITH =,geo_key WITH =,bedrooms WITH =,structure WITH =,utility_code WITH =,valid_during WITH &&,known_during WITH &&) WHERE(status='accepted')
);
CREATE INDEX schedules_lookup_idx ON schedule_rows(authority_id,kind,bedrooms,status);
CREATE INDEX schedules_time_idx ON schedule_rows USING gist(valid_during,known_during);
CREATE TABLE catalog_changes(id bigserial PRIMARY KEY, source_id uuid REFERENCES catalog_sources(id), event text NOT NULL, details jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE properties(
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL REFERENCES tenants(id),
 address text NOT NULL CHECK(length(address) BETWEEN 3 AND 500), city text NOT NULL, state text NOT NULL, zip text NOT NULL,
 bedrooms integer NOT NULL CHECK(bedrooms BETWEEN 0 AND 6), bathrooms numeric(4,1) NOT NULL CHECK(bathrooms>0 AND bathrooms<=20),
 sqft integer NOT NULL CHECK(sqft BETWEEN 100 AND 30000), structure text NOT NULL CHECK(structure IN ('detached','attached','high_rise')),
 condition text NOT NULL CHECK(condition IN ('poor','fair','average','good','renovated')),
 amenities text[] NOT NULL DEFAULT '{}', location geometry(Point,4326),
 geocode_status text NOT NULL CHECK(geocode_status IN ('unverified','user_confirmed','census_interpolated','synthetic')),
 is_demo boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(tenant_id,id)
);
CREATE INDEX properties_tenant_idx ON properties(tenant_id,created_at DESC);
CREATE INDEX properties_location_idx ON properties USING gist(location);
CREATE TABLE comparables(
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL REFERENCES tenants(id), external_id text NOT NULL,
 source_url text NOT NULL, kind text NOT NULL CHECK(kind IN ('asking','executed','synthetic')),
 observed_on date NOT NULL, recorded_at timestamptz NOT NULL DEFAULT now(),
 rent_cents bigint NOT NULL CHECK(rent_cents>0 AND rent_cents<=1000000000),
 bedrooms integer NOT NULL CHECK(bedrooms BETWEEN 0 AND 6), bathrooms numeric(4,1) NOT NULL CHECK(bathrooms>0 AND bathrooms<=20),
 sqft integer NOT NULL CHECK(sqft BETWEEN 100 AND 30000), structure text NOT NULL CHECK(structure IN ('detached','attached','high_rise')),
 condition text NOT NULL CHECK(condition IN ('poor','fair','average','good','renovated')),
 amenities text[] NOT NULL DEFAULT '{}', location geometry(Point,4326) NOT NULL,
 property_ref text NOT NULL, is_demo boolean NOT NULL DEFAULT false,
 UNIQUE(tenant_id,external_id)
);
CREATE INDEX comparable_geo_idx ON comparables USING gist((location::geography));
CREATE INDEX comparable_tenant_idx ON comparables(tenant_id,bedrooms,observed_on);
CREATE TABLE analyses(
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL REFERENCES tenants(id), property_id uuid NOT NULL,
 effective_on date NOT NULL, knowledge_at timestamptz NOT NULL,
 input jsonb NOT NULL, result jsonb NOT NULL, input_sha256 text NOT NULL, result_sha256 text NOT NULL,
 idempotency_key text NOT NULL CHECK(length(idempotency_key)<=160), created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(tenant_id,id), UNIQUE(tenant_id,idempotency_key),
 FOREIGN KEY(tenant_id,property_id) REFERENCES properties(tenant_id,id)
);
CREATE INDEX analyses_portfolio_idx ON analyses(tenant_id,property_id,created_at DESC);
CREATE TABLE jobs(
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL REFERENCES tenants(id), idempotency_key text NOT NULL,
 payload_sha256 text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(tenant_id,id), UNIQUE(tenant_id,idempotency_key)
);
CREATE TABLE job_items(
 id uuid PRIMARY KEY, tenant_id uuid NOT NULL REFERENCES tenants(id), job_id uuid NOT NULL,
 item_index integer NOT NULL, request jsonb NOT NULL,
 status text NOT NULL DEFAULT 'queued' CHECK(status IN ('queued','running','complete','failed')),
 attempts integer NOT NULL DEFAULT 0, lease_until timestamptz, lease_token uuid,
 analysis_id uuid, error text, updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(job_id,item_index), FOREIGN KEY(tenant_id,job_id) REFERENCES jobs(tenant_id,id),
 FOREIGN KEY(tenant_id,analysis_id) REFERENCES analyses(tenant_id,id)
);
CREATE INDEX queue_idx ON job_items(tenant_id,status,lease_until);
CREATE TABLE audit_events(id bigserial PRIMARY KEY, tenant_id uuid NOT NULL REFERENCES tenants(id), action text NOT NULL, object_id text NOT NULL, details jsonb NOT NULL DEFAULT '{}', occurred_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX audit_tenant_idx ON audit_events(tenant_id,occurred_at DESC);
CREATE FUNCTION reject_mutation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'Append-only record: create a new version'; END $$;
CREATE TRIGGER immutable_analyses BEFORE UPDATE OR DELETE ON analyses FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER immutable_sources BEFORE UPDATE OR DELETE ON catalog_sources FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE TRIGGER immutable_audit BEFORE UPDATE OR DELETE ON audit_events FOR EACH ROW EXECUTE FUNCTION reject_mutation();
CREATE FUNCTION tenant_context() RETURNS uuid LANGUAGE sql STABLE AS $$ SELECT nullif(current_setting('app.tenant_id',true),'')::uuid $$;
DO $$ DECLARE t text; BEGIN FOREACH t IN ARRAY ARRAY['properties','comparables','analyses','jobs','job_items','audit_events'] LOOP
 EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY',t);
 EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY',t);
 EXECUTE format('CREATE POLICY tenant_isolation ON %I USING (tenant_id=tenant_context()) WITH CHECK (tenant_id=tenant_context())',t);
END LOOP; END $$;
