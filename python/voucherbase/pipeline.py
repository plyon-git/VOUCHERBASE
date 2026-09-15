"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914.
Migrations, reviewed catalog imports and tenant provisioning. No automatic PHA approval.
"""
import hashlib, json, os, re, secrets, uuid
from datetime import date, datetime, timezone, timedelta
from pathlib import Path

def digest(obj): return hashlib.sha256(json.dumps(obj,sort_keys=True,separators=(",",":"),allow_nan=False).encode()).hexdigest()
def uid(s): return str(uuid.uuid5(uuid.NAMESPACE_URL,"voucherbase:"+s))
def connect():
    import psycopg
    return psycopg.connect(autocommit=False)

def validate_bundle(b):
    if b.get("category") not in ("official","synthetic","user"): raise ValueError("Source category required")
    if not b.get("rows") or len(b["rows"])>50000:raise ValueError("1..50000 rows required")
    authority=b["authority"]
    if not re.fullmatch(r"[A-Za-z0-9_-]{2,32}",authority["id"]):raise ValueError("Invalid authority ID")
    if b["category"]=="synthetic" and not authority.get("is_demo"):raise ValueError("Synthetic data requires demo authority")
    start=date.fromisoformat(b["valid_from"])
    if b.get("valid_until") and date.fromisoformat(b["valid_until"])<=start:raise ValueError("Invalid effective interval")
    if not b["url"].startswith("https://"):raise ValueError("HTTPS source URL required")
    keys=set()
    for row in b["rows"]:
        if row["kind"] not in ("payment_standard","utility","fmr","safmr"):raise ValueError("Invalid kind")
        if not isinstance(row["bedrooms"],int) or isinstance(row["bedrooms"],bool) or not 0<=row["bedrooms"]<=6:raise ValueError("Invalid bedrooms")
        if isinstance(row["value_cents"],bool) or not isinstance(row["value_cents"],int) or not 0<=row["value_cents"]<=1_000_000_000_000:raise ValueError("Integer cents required")
        key=tuple(row.get(x,"*") for x in ("kind","geo_key","bedrooms","structure","utility_code"))
        if key in keys:raise ValueError("Duplicate schedule key")
        keys.add(key)
    return b

def migrate(root):
    from psycopg import sql
    with connect() as conn:
        conn.execute("SELECT pg_advisory_xact_lock(704421)")
        conn.execute("CREATE TABLE IF NOT EXISTS schema_migrations(name text PRIMARY KEY,sha256 text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now())")
        for p in sorted((root/"sql/migrations").glob("*.sql")):
            checksum=hashlib.sha256(p.read_bytes()).hexdigest()
            old=conn.execute("SELECT sha256 FROM schema_migrations WHERE name=%s",(p.name,)).fetchone()
            if old:
                if old[0]!=checksum:raise ValueError("Already-applied migration changed: "+p.name)
                continue
            conn.execute(p.read_text())
            conn.execute("INSERT INTO schema_migrations(name,sha256) VALUES(%s,%s)",(p.name,checksum))
        for role,env in [("vb_app","VB_DB_APP_PASSWORD"),("vb_ingest","VB_DB_INGEST_PASSWORD")]:
            password=os.environ[env]
            if len(password)<24:raise ValueError("Database passwords must have at least 24 characters")
            if not conn.execute("SELECT 1 FROM pg_roles WHERE rolname=%s",(role,)).fetchone():conn.execute(sql.SQL("CREATE ROLE {} LOGIN NOSUPERUSER NOBYPASSRLS").format(sql.Identifier(role)))
            conn.execute(sql.SQL("ALTER ROLE {} PASSWORD {}").format(sql.Identifier(role),sql.Literal(password)))
        conn.execute("GRANT USAGE ON SCHEMA public TO vb_app,vb_ingest")
        conn.execute("GRANT SELECT ON authorities,catalog_sources,schedule_rows,catalog_changes TO vb_app")
        conn.execute("GRANT SELECT,INSERT ON properties,comparables,analyses,jobs,audit_events TO vb_app")
        conn.execute("GRANT SELECT,INSERT,UPDATE ON job_items TO vb_app")
        conn.execute("GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO vb_app,vb_ingest")
        conn.execute("GRANT EXECUTE ON FUNCTION authenticate_key(text) TO vb_app")
        conn.execute("GRANT SELECT,INSERT,UPDATE ON authorities,schedule_rows TO vb_ingest")
        conn.execute("GRANT SELECT,INSERT ON catalog_sources,catalog_changes TO vb_ingest")
        conn.execute("CREATE OR REPLACE FUNCTION work_tenants() RETURNS TABLE(id uuid) LANGUAGE sql STABLE SECURITY DEFINER SET search_path=public,pg_temp AS 'SELECT DISTINCT tenant_id FROM job_items WHERE status IN (''queued'',''running'') LIMIT 100'")
        conn.execute("REVOKE ALL ON FUNCTION work_tenants() FROM PUBLIC")
        conn.execute("GRANT EXECUTE ON FUNCTION work_tenants() TO vb_app")

def import_bundle(b,reviewer=None,accept=False,supersede=False):
    from psycopg.types.json import Jsonb
    validate_bundle(b)
    if accept and (not reviewer or len(reviewer.strip())<3):raise ValueError("Acceptance requires a named reviewer")
    checksum=digest(b); sid=uid(b["external_key"]+":"+checksum)
    with connect() as conn:
        conn.execute("SELECT pg_advisory_xact_lock(704422)")
        old=conn.execute("SELECT snapshot_sha256 FROM catalog_sources WHERE external_key=%s",(b["external_key"],)).fetchone()
        if old:
            if old[0]!=checksum:raise ValueError("Source key already exists with different bytes; give the revision a new external_key")
            return {"status":"unchanged","source_id":sid}
        a=b["authority"]
        conn.execute("INSERT INTO authorities(id,name,state,is_demo) VALUES(%s,%s,%s,%s) ON CONFLICT(id) DO NOTHING",(a["id"],a["name"],a["state"],a.get("is_demo",False)))
        existing=conn.execute("SELECT is_demo FROM authorities WHERE id=%s",(a["id"],)).fetchone()
        if existing[0]!=a.get("is_demo",False):raise ValueError("Authority demo classification conflict")
        now=datetime.now(timezone.utc)
        conn.execute("INSERT INTO catalog_sources(id,external_key,title,url,category,snapshot_sha256,document_sha256,retrieved_at,reviewed_by,reviewed_at,snapshot,notes) VALUES(%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s,%s)",(sid,b["external_key"],b["title"],b["url"],b["category"],checksum,b.get("document_sha256"),now,reviewer,now if accept else None,Jsonb(b),b.get("notes","")))
        for row in b["rows"]:
            key=(a["id"],row["kind"],row.get("geo_key","*"),row["bedrooms"],row.get("structure","any"),row.get("utility_code","none"))
            if accept:
                overlaps=conn.execute("SELECT id,valid_during,value_cents,source_id FROM schedule_rows WHERE authority_id=%s AND kind=%s AND geo_key=%s AND bedrooms=%s AND structure=%s AND utility_code=%s AND status='accepted' AND upper_inf(known_during) AND valid_during && daterange(%s,%s,'[)')",key+(b["valid_from"],b.get("valid_until"))).fetchall()
                new_start=date.fromisoformat(b["valid_from"])
                new_end=date.fromisoformat(b["valid_until"]) if b.get("valid_until") else None
                for oid,interval,old_value,old_source in overlaps:
                    if not supersede: raise ValueError("Accepted overlapping facts exist. Explicit --supersede and reviewer required")
                    conn.execute("UPDATE schedule_rows SET known_during=tstzrange(lower(known_during),%s,'[)') WHERE id=%s",(now,oid))
                    # Retain non-overlapping portions as newly-known slices of the OLD source.
                    parts=[]
                    if interval.lower < new_start:parts.append((interval.lower,new_start))
                    if new_end is not None and (interval.upper is None or new_end < interval.upper):parts.append((new_end,interval.upper))
                    for lo,hi in parts:
                        conn.execute("INSERT INTO schedule_rows(id,authority_id,kind,geo_key,bedrooms,structure,utility_code,value_cents,valid_during,known_during,source_id,status) VALUES(%s,%s,%s,%s,%s,%s,%s,%s,daterange(%s,%s,'[)'),tstzrange(%s,NULL,'[)'),%s,'accepted')",(str(uuid.uuid4()),)+key+(old_value,lo,hi,now,old_source))
            conn.execute("INSERT INTO schedule_rows(id,authority_id,kind,geo_key,bedrooms,structure,utility_code,value_cents,valid_during,known_during,source_id,status) VALUES(%s,%s,%s,%s,%s,%s,%s,%s,daterange(%s,%s,'[)'),tstzrange(%s,NULL,'[)'),%s,%s)",(uid(sid+":"+json.dumps(key)),)+key+(row["value_cents"],b["valid_from"],b.get("valid_until"),now,sid,"accepted" if accept else "pending"))
        conn.execute("INSERT INTO catalog_changes(source_id,event,details) VALUES(%s,%s,%s)",(sid,"accepted" if accept else "pending_review",Jsonb({"rows":len(b["rows"]),"reviewer":reviewer})))
    return {"status":"accepted" if accept else "pending_review","source_id":sid,"rows":len(b["rows"])}

def tenant(name,token=None,role="owner"):
    token=token or secrets.token_urlsafe(36)
    if len(token)<32 or role not in ("owner","viewer"):raise ValueError("Invalid token/role")
    tid=uid("tenant:"+name)
    with connect() as conn:
        conn.execute("INSERT INTO tenants(id,name) VALUES(%s,%s) ON CONFLICT DO NOTHING",(tid,name))
        conn.execute("INSERT INTO api_keys(id,tenant_id,label,token_hash,role) VALUES(%s,%s,%s,%s,%s) ON CONFLICT(token_hash) DO NOTHING",(str(uuid.uuid4()),tid,name,hashlib.sha256(token.encode()).hexdigest(),role))
    return {"tenant_id":tid,"token":token,"role":role}

def seed(root):
    import_bundle(json.loads((root/"data/denver_2026.json").read_text()),"Build-time visual transcription review; reconfirm locally",True)
    demo=os.getenv("VB_DEMO","1")=="1"
    account=tenant("Local landlord",os.environ["VB_API_TOKEN"])
    if not demo:return
    import_bundle(json.loads((root/"data/demo_schedules.json").read_text()),"Synthetic fixture",True)
    tid=account["tenant_id"]
    with connect() as conn:
        # Synthetic polygon is explicitly marked and never used for the real DHA authority.
        conn.execute("UPDATE authorities SET boundary=ST_Multi(ST_GeomFromText('POLYGON((-105.10 39.65,-104.90 39.65,-104.90 39.85,-105.10 39.85,-105.10 39.65))',4326)),boundary_source='synthetic fixture' WHERE id='DEMO'")
        conn.execute("SELECT set_config('app.tenant_id',%s,true)",(tid,))
        conn.execute("INSERT INTO properties(id,tenant_id,address,city,state,zip,bedrooms,bathrooms,sqft,structure,condition,amenities,location,geocode_status,is_demo) VALUES(%s,%s,'100 Example Lane','Denver','CO','80204',3,2,1500,'detached','good',ARRAY['parking','laundry'],ST_SetSRID(ST_MakePoint(-105.01,39.737),4326),'synthetic',true) ON CONFLICT DO NOTHING",(uid("demo-property"),tid))
        for i,rent in enumerate([210000,220000,225000,230000,235000,245000]):
            conn.execute("INSERT INTO comparables(id,tenant_id,external_id,source_url,kind,observed_on,rent_cents,bedrooms,bathrooms,sqft,structure,condition,amenities,location,property_ref,is_demo) VALUES(%s,%s,%s,'https://example.invalid/synthetic','synthetic',%s,%s,3,2,%s,'detached','good',ARRAY['parking','laundry'],ST_SetSRID(ST_MakePoint(%s,%s),4326),%s,true) ON CONFLICT DO NOTHING",(uid("demo-comp:"+str(i)),tid,"demo-"+str(i),date.today()-timedelta(days=15+i*8),rent,1400+i*30,-105.012+i*.001,39.738+i*.001,"synthetic-property-"+str(i)))

def revoke(key_id):
    with connect() as conn:conn.execute("UPDATE api_keys SET revoked_at=now() WHERE id=%s",(key_id,))

def approve(external_key,reviewer,supersede=False):
    """Create a separate immutable reviewed snapshot; the pending submission is retained."""
    import copy
    with connect() as conn:
        row=conn.execute("SELECT snapshot FROM catalog_sources WHERE external_key=%s AND reviewed_at IS NULL",(external_key,)).fetchone()
        if not row:raise ValueError("Pending source not found")
        b=copy.deepcopy(row[0])
    b["external_key"]=external_key+"-accepted-"+datetime.now(timezone.utc).strftime("%Y%m%d%H%M%S%f")
    b["notes"]=b.get("notes","")+" Reviewed successor of pending source "+external_key
    return import_bundle(b,reviewer,True,supersede)
