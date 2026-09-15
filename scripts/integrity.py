#!/usr/bin/env python3
"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914. Source checksums, not signatures or copy prevention."""
import argparse,hashlib,json,sys
from pathlib import Path
OWNER="Parrish Lyon";MARK="PL-VOUCHERBASE-20260914"
EXCLUDE={".git","__pycache__",".venv","node_modules","target","artifacts","imports"}
def entries(root):
 out={}
 for p in root.rglob("*"):
  rel=p.relative_to(root)
  if any(x in EXCLUDE for x in rel.parts) or p.name in {"WATERMARK.json",".env","source-package.b64","bootstrap-source.yml","artifacts-ci.log"} or p.suffix==".pyc":continue
  if p.is_symlink():raise ValueError("Symlink forbidden: "+str(rel))
  if p.is_file():out[rel.as_posix()]=hashlib.sha256(p.read_bytes()).hexdigest()
 return dict(sorted(out.items()))
def verify(root):
 m=json.loads((root/"WATERMARK.json").read_text())
 if m.get("owner")!=OWNER or m.get("watermark")!=MARK:raise ValueError("Ownership metadata mismatch")
 actual=entries(root)
 changed=sorted(k for k in set(actual)|set(m["files"]) if actual.get(k)!=m["files"].get(k))
 if changed:raise ValueError("Changed/missing/added files: "+", ".join(changed))
 return len(actual)
def main():
 p=argparse.ArgumentParser();p.add_argument("--root",default=str(Path(__file__).resolve().parents[1]));p.add_argument("--write",action="store_true",help="Explicitly approve current files as a new baseline. Never run blindly.")
 a=p.parse_args();root=Path(a.root).resolve()
 if a.write:
  (root/"WATERMARK.json").write_text(json.dumps({"owner":OWNER,"watermark":MARK,"algorithm":"SHA-256","notice":"Unsigned change-detection manifest. Trust the approved Git commit or independently retained checksum, not a self-modified manifest.","files":entries(root)},indent=2)+"\n")
 else:print(f"Verified {verify(root)} files | {OWNER} | {MARK}")
if __name__=="__main__":
 try:main()
 except (ValueError,OSError,KeyError,json.JSONDecodeError) as e:print(str(e),file=sys.stderr);sys.exit(1)
