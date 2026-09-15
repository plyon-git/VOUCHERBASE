"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914."""
import argparse,json,os
from pathlib import Path
from .pipeline import migrate,seed,tenant,import_bundle,revoke,approve
from .providers import hud_snapshot
from .model import rolling_group_evaluation

def main():
    p=argparse.ArgumentParser(description="VOUCHERBASE reviewed-data and account administration")
    s=p.add_subparsers(dest="command",required=True)
    s.add_parser("bootstrap");s.add_parser("migrate")
    t=s.add_parser("tenant");t.add_argument("--name",required=True);t.add_argument("--role",choices=["owner","viewer"],default="owner")
    r=s.add_parser("revoke");r.add_argument("--key-id",required=True)
    i=s.add_parser("import");i.add_argument("file");i.add_argument("--accept",action="store_true");i.add_argument("--reviewer");i.add_argument("--supersede",action="store_true")
    ap=s.add_parser("approve");ap.add_argument("--external-key",required=True);ap.add_argument("--reviewer",required=True);ap.add_argument("--supersede",action="store_true")
    h=s.add_parser("hud");h.add_argument("--entity",required=True);h.add_argument("--year",required=True,type=int);h.add_argument("--output",required=True);h.add_argument("--valid-from",required=True);h.add_argument("--valid-until",required=True)
    e=s.add_parser("evaluate");e.add_argument("file")
    a=p.parse_args();root=Path(os.getenv("VB_ROOT","/workspace"))
    if a.command in ("bootstrap","migrate"):
        migrate(root)
        if a.command=="bootstrap":seed(root)
        print(json.dumps({"status":"ready"}))
    elif a.command=="tenant":print(json.dumps(tenant(a.name,role=a.role)))
    elif a.command=="revoke":revoke(a.key_id);print('{"status":"revoked"}')
    elif a.command=="import":print(json.dumps(import_bundle(json.loads(Path(a.file).read_text()),a.reviewer,a.accept,a.supersede)))
    elif a.command=="approve":print(json.dumps(approve(a.external_key,a.reviewer,a.supersede)))
    elif a.command=="hud":
        out=hud_snapshot(a.entity,a.year)
        bundle={"external_key":f"hud-{a.entity}-{a.year}-{out['raw_sha256'][:12]}","title":f"HUD FMR benchmark {a.entity} FY{a.year}","authority":{"id":"HUD","name":"HUD benchmark catalog","state":"US","is_demo":False},"category":"official","url":out["source_url"],"valid_from":a.valid_from,"valid_until":a.valid_until,"document_sha256":out["raw_sha256"],"notes":"API benchmark, not a PHA payment standard. Effective interval supplied by importer for review.","rows":out["rows"],"raw_payload":out["payload"]}
        from .pipeline import validate_bundle
        validate_bundle(bundle);Path(a.output).write_text(json.dumps(bundle,indent=2));print("Reviewable HUD benchmark bundle saved. Not yet accepted or a PHA payment standard.")
    elif a.command=="evaluate":print(json.dumps(rolling_group_evaluation(json.loads(Path(a.file).read_text())),indent=2))
if __name__=="__main__":main()
