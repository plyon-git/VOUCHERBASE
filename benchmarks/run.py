#!/usr/bin/env python3
"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914. Measure, do not invent throughput."""
import argparse,json,platform,subprocess,time
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument("--iterations",type=int,default=100);a=p.parse_args()
if not 1<=a.iterations<=10000:raise SystemExit("iterations must be 1..10000")
request={"authority_confirmed":True,"unit_bedrooms":3,"unit_standard_cents":273400,"utility_allowance_cents":23700,"proposed_rent_cents":240000,"scenarios":[]}
exe=Path("target/release/rent-core")
if not exe.exists():raise SystemExit("First run cargo build --release -p rent-core")
start=time.perf_counter();failed=0
for _ in range(a.iterations):
 r=subprocess.run([str(exe.resolve())],input=json.dumps(request),text=True,capture_output=True);failed+=r.returncode!=0
seconds=time.perf_counter()-start
out={"benchmark":"CLI process-per-calculation (includes startup overhead)","iterations":a.iterations,"elapsed_seconds":seconds,"failures":failed,"calculations_per_second":a.iterations/seconds,"environment":platform.platform(),"synthetic":True}
Path("artifacts").mkdir(exist_ok=True);Path("artifacts/benchmark.json").write_text(json.dumps(out,indent=2));print(json.dumps(out,indent=2))
