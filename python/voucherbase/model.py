"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914.
Transparent comparable selection. Descriptive ranges are NOT confidence intervals.
No demographics, voucher participation, tenant identity or protected characteristics.
"""
from datetime import date, datetime, timezone
from math import asin, cos, radians, sin, sqrt, isfinite
from statistics import median

CONDITIONS = {"poor": 0, "fair": 1, "average": 2, "good": 3, "renovated": 4}

def distance_m(a, b):
    if not all(isfinite(x) for x in (*a,*b)) or abs(a[0])>90 or abs(b[0])>90 or abs(a[1])>180 or abs(b[1])>180: raise ValueError("Invalid coordinates")
    p1, p2 = radians(a[0]), radians(b[0])
    dlat, dlon = p2 - p1, radians(b[1] - a[1])
    v = sin(dlat / 2)**2 + cos(p1)*cos(p2)*sin(dlon/2)**2
    return 6371008.8 * 2 * asin(min(1.0, sqrt(v)))

def timestamp(s):
    t = datetime.fromisoformat(s.replace("Z", "+00:00"))
    if t.tzinfo is None: raise ValueError("Knowledge timestamps need a timezone")
    return t.astimezone(timezone.utc)

def compare(subject, candidates, effective_on, knowledge_at, kind="executed"):
    if kind not in ("executed", "asking", "synthetic"): raise ValueError("Invalid comparable kind")
    if len(candidates) > 2000: raise ValueError("At most 2000 candidate records")
    if subject.get("latitude") is None or subject.get("longitude") is None:
        return {"status": "location_unknown", "selected": [], "excluded": [], "range_cents": None, "median_cents": None, "method": "comparable-median/1.0.0", "kind":kind}
    effective, known = date.fromisoformat(effective_on), timestamp(knowledge_at)
    chosen, excluded, seen = [], [], set()
    for c in sorted(candidates, key=lambda x: (x.get("observed_on", ""), x.get("id", "")), reverse=True):
        why = None
        try:
            observed = date.fromisoformat(c["observed_on"])
            age = (effective - observed).days
            d = distance_m((subject["latitude"],subject["longitude"]),(c["latitude"],c["longitude"]))
            if c["property_ref"] == subject.get("id") or c["property_ref"] in seen: why = "same_or_duplicate_property"
            elif c["kind"] != kind: why = "different_observation_kind"
            elif bool(c.get("is_demo")) != bool(subject.get("is_demo")): why = "demo_live_separation"
            elif timestamp(c["recorded_at"]) > known: why = "not_yet_known"
            elif age < 0 or age > 180: why = "outside_observation_window"
            elif c["bedrooms"] != subject["bedrooms"] or c["structure"] != subject["structure"]: why = "bedroom_or_structure_mismatch"
            elif c["condition"] != subject["condition"]: why = "condition_mismatch"
            elif not 0.7 <= c["sqft"] / subject["sqft"] <= 1.3: why = "size_mismatch"
            elif abs(c["bathrooms"]-subject["bathrooms"]) > 0.5: why = "bathroom_mismatch"
            elif d > 3000: why = "outside_3km_radius"
            elif isinstance(c["rent_cents"],bool) or not isinstance(c["rent_cents"],int) or not 0 < c["rent_cents"] <= 1_000_000_000: why = "invalid_rent"
            if why is None:
                seen.add(c["property_ref"])
                sa, ca = set(subject.get("amenities",[])), set(c.get("amenities",[]))
                similarity = len(sa & ca)/max(len(sa | ca),1)
                score = d/3000 + age/180 + abs(c["sqft"]-subject["sqft"])/subject["sqft"] + (1-similarity)*0.25
                chosen.append({**c,"distance_m":round(d,1),"age_days":age,"selection_score":round(score,4)})
        except (KeyError, ValueError, ZeroDivisionError, TypeError): why = "invalid_observation"
        if why: excluded.append({"id":c.get("id"),"reason":why})
    chosen = sorted(chosen,key=lambda c:(c["selection_score"],c["id"]))[:12]
    rents = sorted(c["rent_cents"] for c in chosen)
    enough = len(rents)>=3
    return {"status":"sufficient_descriptive_sample" if enough else "insufficient_comparables", "selected":chosen,"excluded":excluded,
        "range_cents":[rents[0],rents[-1]] if enough else None,"median_cents":int(median(rents)) if enough else None,
        "kind":kind,"method":"comparable-median/1.0.0","sample_size":len(rents),
        "limitations":["Selected-observation minimum/maximum, not a confidence or prediction interval.","No property-feature price adjustments are invented.","Asking rents are not executed leases. This is not a PHA rent-reasonableness determination."]}

def rolling_group_evaluation(observations):
    """One held-out property at a time; train only on strictly older OTHER properties."""
    errors=[]
    for target in sorted(observations,key=lambda c:c["observed_on"]):
        past=[c for c in observations if c["observed_on"]<target["observed_on"] and c["property_ref"]!=target["property_ref"]]
        if len(past)<3: continue
        # Evaluate the SAME filters as serving, with the target's recording time.
        out=compare({**target,"id":target["property_ref"]},past,target["observed_on"],target["recorded_at"],target["kind"])
        if out["median_cents"] is not None: errors.append(abs(out["median_cents"]-target["rent_cents"]))
    return {"evaluation":"time-ordered-property-holdout/1.0.0","test_predictions":len(errors),"mae_cents":sum(errors)/len(errors) if errors else None,"synthetic_only":all(c.get("is_demo",False) for c in observations),"limitations":"Not a nationally validated rent model. Sparse histories can yield no evaluable predictions."}
