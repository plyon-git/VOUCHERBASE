"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914.
Run against a disposable local/CI database, as the admin service. Never uses landlord data.
"""
import copy,os,unittest,uuid
from datetime import date,datetime,timezone,timedelta
import psycopg
from voucherbase.pipeline import tenant,import_bundle,approve
class DatabaseTests(unittest.TestCase):
 @classmethod
 def setUpClass(cls):
  cls.a=tenant("Isolation test A")["tenant_id"];cls.b=tenant("Isolation test B")["tenant_id"]
  cls.pid=str(uuid.uuid4())
  with psycopg.connect() as c:
   c.execute("INSERT INTO properties(id,tenant_id,address,city,state,zip,bedrooms,bathrooms,sqft,structure,condition,geocode_status) VALUES(%s,%s,'Isolation fixture','Denver','CO','80204',3,2,1500,'detached','good','unverified')",(cls.pid,cls.a))
 def app(self):return psycopg.connect(user="vb_app",password=os.environ["VB_DB_APP_PASSWORD"])
 def test_nonprivileged_role(self):
  with self.app() as c:self.assertEqual(c.execute("SELECT rolsuper,rolbypassrls FROM pg_roles WHERE rolname=current_user").fetchone(),(False,False))
 def test_default_denies_all(self):
  with self.app() as c:self.assertEqual(c.execute("SELECT count(*) FROM properties").fetchone()[0],0)
 def test_other_tenant_hidden(self):
  with self.app() as c:
   c.execute("SELECT set_config('app.tenant_id',%s,true)",(self.b,));self.assertEqual(c.execute("SELECT count(*) FROM properties WHERE id=%s",(self.pid,)).fetchone()[0],0)
 def test_own_tenant_visible(self):
  with self.app() as c:
   c.execute("SELECT set_config('app.tenant_id',%s,true)",(self.a,));self.assertEqual(c.execute("SELECT count(*) FROM properties WHERE id=%s",(self.pid,)).fetchone()[0],1)
 def test_cross_tenant_insert_rejected(self):
  with self.app() as c:
   c.execute("SELECT set_config('app.tenant_id',%s,true)",(self.b,))
   with self.assertRaises(psycopg.errors.InsufficientPrivilege):
    c.execute("INSERT INTO audit_events(tenant_id,action,object_id) VALUES(%s,'bad','test')",(self.a,))
   c.rollback()
 def test_catalog_not_writable_by_app(self):
  with self.app() as c:
   with self.assertRaises(psycopg.errors.InsufficientPrivilege):c.execute("DELETE FROM schedule_rows")
   c.rollback()
 def test_sources_append_only(self):
  with psycopg.connect() as c:
   with self.assertRaises(psycopg.errors.RaiseException):c.execute("UPDATE catalog_sources SET title='changed'")
   c.rollback()
 def test_review_and_bitemporal_split(self):
  key="sql-test-"+uuid.uuid4().hex
  b={"external_key":key,"authority":{"id":"SQLTEST","name":"Synthetic SQL test authority","state":"CO","is_demo":True},"title":"SQL fixture","url":"https://example.invalid/sql","category":"synthetic","valid_from":"2026-01-01","valid_until":None,"rows":[{"kind":"payment_standard","bedrooms":3,"value_cents":200000}]}
  pending=import_bundle(b)
  with psycopg.connect() as c:self.assertEqual(c.execute("SELECT count(*) FROM schedule_rows WHERE source_id=%s AND status='accepted'",(pending["source_id"],)).fetchone()[0],0)
  accepted=approve(key,"CI reviewer")
  with psycopg.connect() as c:known=c.execute("SELECT lower(known_during) FROM schedule_rows WHERE source_id=%s",(accepted["source_id"],)).fetchone()[0]
  newer=copy.deepcopy(b);newer["external_key"]=key+"-revision";newer["valid_from"]="2026-07-01";newer["rows"][0]["value_cents"]=230000
  with self.assertRaises(ValueError):import_bundle(newer,"CI reviewer",True)
  import_bundle(newer,"CI reviewer",True,True)
  with psycopg.connect() as c:
   query="SELECT value_cents FROM schedule_rows WHERE authority_id='SQLTEST' AND status='accepted' AND valid_during @> %s::date AND known_during @> %s::timestamptz"
   self.assertEqual(c.execute(query,("2026-08-01",known)).fetchone()[0],200000)
   self.assertEqual(c.execute(query,("2026-08-01",datetime.now(timezone.utc))).fetchone()[0],230000)
   self.assertEqual(c.execute(query,("2026-02-01",datetime.now(timezone.utc))).fetchone()[0],200000)
 def test_exclusion_constraint(self):
  with psycopg.connect() as c:
   with self.assertRaises(psycopg.errors.ExclusionViolation):c.execute("INSERT INTO schedule_rows SELECT %s,authority_id,kind,geo_key,bedrooms,structure,utility_code,value_cents,valid_during,known_during,source_id,status FROM schedule_rows WHERE authority_id='DHA' AND kind='payment_standard' AND bedrooms=3 LIMIT 1",(str(uuid.uuid4()),))
   c.rollback()
 def test_spatial_fixture_only(self):
  with psycopg.connect() as c:
   self.assertEqual(c.execute("SELECT ST_Covers(boundary,ST_SetSRID(ST_MakePoint(-105.01,39.737),4326)),boundary_verified FROM authorities WHERE id='DEMO'").fetchone(),(True,False))
   self.assertIsNone(c.execute("SELECT boundary FROM authorities WHERE id='DHA'").fetchone()[0])
if __name__=="__main__":unittest.main(verbosity=2)
