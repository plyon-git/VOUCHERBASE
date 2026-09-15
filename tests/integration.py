"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914. Four-service integration checks."""
import copy,json,os,subprocess,time,unittest,uuid
from datetime import date,datetime,timezone
from pathlib import Path
from urllib.request import Request,urlopen
from urllib.error import HTTPError
ROOT=Path(__file__).resolve().parents[1]
ENV=dict(line.split("=",1) for line in (ROOT/".env").read_text().splitlines() if "=" in line and not line.startswith("#"))
TOKEN=ENV["VB_API_TOKEN"];BASE="http://localhost:8080/api/v1"
def api(path,method="GET",body=None,token=TOKEN,headers=None):
 h={"Authorization":"Bearer "+token,**(headers or {})}
 if body is not None and not isinstance(body,str):body=json.dumps(body).encode();h["Content-Type"]="application/json"
 elif isinstance(body,str):body=body.encode()
 try:
  with urlopen(Request(BASE+path,data=body,method=method,headers=h),timeout=50) as r:return r.status,json.loads(r.read())
 except HTTPError as e:return e.code,json.loads(e.read())
def admin(code):
 out=subprocess.run(["docker","compose","run","--rm","-T","admin","python","-c",code],cwd=ROOT,capture_output=True,text=True,check=True)
 return json.loads(out.stdout.strip().splitlines()[-1])
class IntegrationTests(unittest.TestCase):
 @classmethod
 def setUpClass(cls):
  for _ in range(90):
   try:
    status,props=api("/properties")
    if status==200:break
   except OSError:pass
   time.sleep(2)
  else:raise AssertionError("API readiness failed")
  cls.demo=next(p for p in props if p["address"]=="100 Example Lane")
  cls.req={"property_id":cls.demo["id"],"effective_on":date.today().isoformat(),"knowledge_at":"","authority_id":"DEMO","authority_confirmed":True,"voucher_bedrooms":None,"utilities":{"complete":True,"items":["heat_gas","other_electric","water_sewer"]},"proposed_rent_cents":230000,"underwriting":None,"scenarios":[],"comparable_kind":"synthetic","benchmark_entity":""}
  cls.other=admin("import json;from voucherbase.pipeline import tenant;print(json.dumps(tenant('API isolation fixture')))")
  cls.viewer=admin("import json;from voucherbase.pipeline import tenant;print(json.dumps(tenant('Local landlord',role='viewer')))")
 def analyze(self,request=None,key=None):return api("/analyses","POST",request or self.req,headers={"Idempotency-Key":key or str(uuid.uuid4())})
 def test_01_end_to_end(self):
  status,out=self.analyze();self.assertEqual(status,201,out);r=out["result"]
  self.assertEqual(r["calculation"]["planning_contract_reference_cents"],205000)
  self.assertEqual(r["calculation"]["pha_approval"],"not_obtained")
  self.assertEqual(r["market"]["median_cents"],227500)
  self.assertEqual(r["watermark"],"PL-VOUCHERBASE-20260914")
  status,replay=api("/analyses/"+out["id"]+"/replay","POST",{});self.assertEqual(status,200,replay);self.assertTrue(replay["matched"],replay)
 def test_02_idempotency(self):
  key=str(uuid.uuid4());s,a=self.analyze(key=key);self.assertEqual(s,201,a);s,b=self.analyze(key=key);self.assertEqual(s,200,b);self.assertEqual(a["id"],b["id"])
  req=copy.deepcopy(self.req);req["proposed_rent_cents"]=250000;self.assertEqual(self.analyze(req,key)[0],409)
 def test_03_missing_utility(self):
  req=copy.deepcopy(self.req);req["utilities"]["complete"]=False;s,out=self.analyze(req);self.assertEqual(s,201,out);self.assertIsNone(out["result"]["calculation"]["planning_contract_reference_cents"])
 def test_04_unknown_authority(self):
  req=copy.deepcopy(self.req);req["authority_confirmed"]=False;s,out=self.analyze(req);self.assertEqual(s,201,out);self.assertIsNone(out["result"]["calculation"]["payment_standard_cents"])
 def test_05_historical_knowledge(self):
  req=copy.deepcopy(self.req);req["knowledge_at"]="2025-01-01T00:00:00Z";s,out=self.analyze(req);self.assertEqual(s,201,out);self.assertIsNone(out["result"]["calculation"]["payment_standard_cents"])
 def test_06_official_pilot(self):
  p={k:v for k,v in self.demo.items() if k not in ("latest_analysis_id","latest_result","id")};p.update(address="Unverified property fixture",is_demo=False,geocode_status="user_confirmed")
  s,p=api("/properties","POST",p);self.assertEqual(s,201,p)
  req=copy.deepcopy(self.req);req.update(property_id=p["id"],authority_id="DHA",effective_on="2026-09-14",comparable_kind="executed")
  s,out=self.analyze(req);self.assertEqual(s,201,out);c=out["result"]["calculation"];self.assertEqual(c["payment_standard_cents"],273400);self.assertEqual(c["utility_allowance_cents"],23700);self.assertEqual(c["planning_contract_reference_cents"],249700)
  self.assertIsNone(out["result"]["market"]["median_cents"])
 def test_07_tenant_isolation(self):
  s,props=api("/properties",token=self.other["token"]);self.assertEqual(s,200);self.assertEqual(props,[])
  _,out=self.analyze();self.assertEqual(api("/analyses/"+out["id"],token=self.other["token"])[0],404)
  self.assertEqual(api("/analyses","POST",self.req,token=self.other["token"],headers={"Idempotency-Key":str(uuid.uuid4())})[0],404)
 def test_08_viewer_cannot_write(self):self.assertEqual(api("/analyses","POST",self.req,token=self.viewer["token"])[0],403)
 def test_09_auth_origin(self):
  self.assertEqual(api("/me",token="bad")[0],401);self.assertEqual(api("/me",headers={"Origin":"https://evil.invalid"})[0],403)
 def test_10_unknown_fields(self):
  req=dict(self.req,unexpected=1);self.assertEqual(self.analyze(req)[0],400)
 def test_11_no_synthetic_live_mix(self):
  req=dict(self.req,authority_id="DHA");self.assertNotEqual(self.analyze(req)[0],201)
 def test_12_jobs(self):
  s,out=api("/jobs","POST",{"items":[self.req,self.req]},headers={"Idempotency-Key":str(uuid.uuid4())});self.assertEqual(s,202,out)
  for _ in range(60):
   s,job=api("/jobs/"+out["id"])
   if job["job"]["complete"]+job["job"]["failed"]==2:break
   time.sleep(1)
  self.assertEqual(job["job"]["complete"],2,job)
 def test_13_csv_atomic(self):
  before=len(api("/properties")[1]);data="address,city,state,zip,bedrooms,bathrooms,sqft,structure,condition\nOkay Address,Denver,CO,80204,3,2,1500,detached,good\nBad,Denver,CO,80204,99,2,1500,detached,good\n"
  self.assertEqual(api("/imports/properties","POST",data,headers={"Content-Type":"text/csv"})[0],422);self.assertEqual(len(api("/properties")[1]),before)
 def test_14_provider_explicit(self):
  s,out=api("/geocode","POST",{"address":"123 Not Transmitted","provider":"census"});self.assertEqual(s,200,out);self.assertEqual(out["status"],"provider_disabled")
 def test_15_source_provenance(self):
  s,cat=api("/catalog");self.assertEqual(s,200);source=next(x for x in cat["sources"] if x["category"]=="official");self.assertEqual(len(source["snapshot_sha256"]),64)
  s,out=api("/sources/"+source["id"]);self.assertEqual(s,200);self.assertIsNone(out["document_sha256"])
if __name__=="__main__":unittest.main(verbosity=2)
