"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914."""
import importlib.util,json,tempfile,unittest
from pathlib import Path
spec=importlib.util.spec_from_file_location("integrity",Path(__file__).resolve().parents[1]/"scripts/integrity.py");m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
class IntegrityTests(unittest.TestCase):
 def setUp(self):self.tmp=tempfile.TemporaryDirectory();self.root=Path(self.tmp.name);(self.root/"example.go").write_text("// Parrish Lyon");(self.root/"WATERMARK.json").write_text(json.dumps({"owner":m.OWNER,"watermark":m.MARK,"files":m.entries(self.root)}))
 def tearDown(self):self.tmp.cleanup()
 def test_good(self):self.assertEqual(m.verify(self.root),1)
 def test_changed(self):
  (self.root/"example.go").write_text("changed")
  with self.assertRaises(ValueError):m.verify(self.root)
 def test_missing(self):
  (self.root/"example.go").unlink()
  with self.assertRaises(ValueError):m.verify(self.root)
 def test_added(self):
  (self.root/"new.go").write_text("new")
  with self.assertRaises(ValueError):m.verify(self.root)
 def test_owner(self):
  p=self.root/"WATERMARK.json";v=json.loads(p.read_text());v["owner"]="changed";p.write_text(json.dumps(v))
  with self.assertRaises(ValueError):m.verify(self.root)
 def test_symlink(self):
  (self.root/"link").symlink_to(self.root/"example.go")
  with self.assertRaises(ValueError):m.verify(self.root)
if __name__=="__main__":unittest.main()
