"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914."""
import copy,json,os,unittest
from pathlib import Path
from unittest.mock import patch
from voucherbase.model import compare,distance_m,rolling_group_evaluation,timestamp
from voucherbase.providers import geocode,money_cents,normalize_hud,fetch_json,hud_snapshot
from voucherbase.pipeline import validate_bundle,digest
ROOT=Path(__file__).resolve().parents[2]
def subject():return dict(id="subject",latitude=39.737,longitude=-105.01,bedrooms=3,bathrooms=2,sqft=1500,structure="detached",condition="good",amenities=["parking"],is_demo=True)
def comps():return [dict(subject(),id=str(i),property_ref="other"+str(i),recorded_at="2026-08-02T12:00:00Z",observed_on="2026-08-01",kind="synthetic",rent_cents=200000+i*10000,source_url="https://example.invalid/fixture") for i in range(5)]
def result(rows=None,sub=None):return compare(sub or subject(),rows if rows is not None else comps(),"2026-09-14","2026-09-14T12:00:00Z","synthetic")
class ModelTests(unittest.TestCase):
 def test_sample(self):self.assertEqual(result()["median_cents"],220000)
 def test_sparse(self):self.assertIsNone(result(comps()[:2])["median_cents"])
 def test_unknown_location(self):s=subject();s["latitude"]=None;self.assertEqual(result(sub=s)["status"],"location_unknown")
 def test_no_future_observation(self):rows=comps();rows[0]["observed_on"]="2026-10-01";self.assertEqual(len(result(rows)["selected"]),4)
 def test_not_yet_known(self):rows=comps();rows[0]["recorded_at"]="2026-10-01T00:00:00Z";self.assertEqual(len(result(rows)["selected"]),4)
 def test_no_duplicate_property(self):rows=comps();rows[1]["property_ref"]=rows[0]["property_ref"];self.assertEqual(len(result(rows)["selected"]),4)
 def test_subject_excluded(self):rows=comps();rows[0]["property_ref"]="subject";self.assertEqual(len(result(rows)["selected"]),4)
 def test_asking_not_executed(self):rows=comps();rows[0]["kind"]="asking";self.assertEqual(len(result(rows)["selected"]),4)
 def test_demo_live(self):rows=comps();rows[0]["is_demo"]=False;self.assertEqual(len(result(rows)["selected"]),4)
 def test_size(self):rows=comps();rows[0]["sqft"]=300;self.assertEqual(len(result(rows)["selected"]),4)
 def test_condition(self):rows=comps();rows[0]["condition"]="poor";self.assertEqual(len(result(rows)["selected"]),4)
 def test_distance(self):self.assertAlmostEqual(distance_m((0,0),(0,0)),0);rows=comps();rows[0]["latitude"]=0;self.assertEqual(len(result(rows)["selected"]),4)
 def test_nonfinite(self):rows=comps();rows[0]["longitude"]=float("nan");self.assertEqual(len(result(rows)["selected"]),4)
 def test_timezone(self):
  with self.assertRaises(ValueError):timestamp("2026-01-01")
 def test_eval_sparse_no_claim(self):self.assertIsNone(rolling_group_evaluation(comps())["mae_cents"])
 def test_range_is_descriptive(self):self.assertEqual(result()["range_cents"],[200000,240000]);self.assertIn("not a confidence",result()["limitations"][0])
class ProviderTests(unittest.TestCase):
 def test_cents(self):self.assertEqual(money_cents("$1,234.56"),123456)
 def test_money_invalid(self):
  for value in (True,"nan","-1","0.001","inf"):
   with self.assertRaises(ValueError):money_cents(value)
 def test_fixed_origin(self):
  with self.assertRaises(ValueError):fetch_json("https://example.com/anything")
 def test_no_embedded_token(self):
  with patch.dict(os.environ,{"HUD_API_TOKEN":""}):
   with self.assertRaises(ValueError):hud_snapshot("METRO00001",2026)
 def test_disabled_geocoder(self):
  with patch.dict(os.environ,{"VB_LIVE_PROVIDERS":"0"}):self.assertEqual(geocode("123 Private Street")["status"],"provider_disabled")
 def test_fixture_never_guesses(self):self.assertEqual(geocode("123 Private Street","fixture")["matches"],[])
 def test_fixture_label(self):self.assertEqual(geocode("100 Example Lane, Denver, CO 80204","fixture")["provider"],"synthetic")
 def test_census_match_provenance(self):
  mock={"result":{"addressMatches":[{"matchedAddress":"Example","coordinates":{"x":-105,"y":39},"addressComponents":{"state":"CO","zip":"80204"}}]}}
  with patch.dict(os.environ,{"VB_LIVE_PROVIDERS":"1"}),patch("voucherbase.providers.fetch_json",return_value=(mock,"abc")):
   self.assertEqual(geocode("123 Example")["matches"][0]["quality"],"census_interpolated")
 def test_hud_separate_benchmarks(self):
  base={k:1000 for k in ["Efficiency","One-Bedroom","Two-Bedroom","Three-Bedroom","Four-Bedroom"]}
  rows=normalize_hud({"data":{"year":"2026","basicdata":[dict(base,zip_code="MSA level"),dict(base,zip_code="80204")]}},2026,"METRO")
  self.assertEqual({r["kind"] for r in rows},{"fmr","safmr"});self.assertEqual(rows[0]["value_cents"],100000)
 def test_hud_wrong_year(self):
  with self.assertRaises(ValueError):normalize_hud({"data":{"year":2025,"basicdata":{}}},2026,"METRO")
 def test_hud_schema_fail_closed(self):
  with self.assertRaises(ValueError):normalize_hud({},2026,"METRO")
class DataTests(unittest.TestCase):
 def setUp(self):self.b=json.loads((ROOT/"data/denver_2026.json").read_text())
 def test_official_transcription(self):validate_bundle(self.b);self.assertEqual([r["value_cents"] for r in self.b["rows"] if r["kind"]=="payment_standard"],[164300,175400,208900,273400,304900,350600,396400])
 def test_no_fake_pdf_hash(self):self.assertIsNone(self.b["document_sha256"])
 def test_duplicate_rejected(self):
  self.b["rows"].append(copy.deepcopy(self.b["rows"][0]))
  with self.assertRaises(ValueError):validate_bundle(self.b)
 def test_bad_effective_window(self):
  self.b["valid_until"]=self.b["valid_from"]
  with self.assertRaises(ValueError):validate_bundle(self.b)
 def test_money_integer(self):
  self.b["rows"][0]["value_cents"]=0.1
  with self.assertRaises(ValueError):validate_bundle(self.b)
 def test_digest_canonical(self):self.assertEqual(digest({"a":1,"b":2}),digest({"b":2,"a":1}))
 def test_demo_separate(self):b=json.loads((ROOT/"data/demo_schedules.json").read_text());validate_bundle(b);self.assertTrue(b["authority"]["is_demo"]);self.assertEqual(b["category"],"synthetic")
if __name__=="__main__":unittest.main()
