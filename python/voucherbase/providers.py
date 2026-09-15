"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914.
Fixed-origin providers, explicit fiscal years, bounded downloads and honest no-match.
"""
import hashlib, json, os, re, time
from decimal import Decimal, InvalidOperation
from urllib.parse import urlencode
from urllib.request import Request, urlopen, HTTPRedirectHandler, build_opener
from urllib.error import HTTPError

class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self,*args,**kwargs): raise ValueError("Provider redirects need operator review")

def fetch_json(url, token=None, attempts=3):
    if not (url.startswith("https://www.huduser.gov/hudapi/public/fmr/") or url.startswith("https://geocoding.geo.census.gov/geocoder/")):
        raise ValueError("Provider origin is not allowed")
    headers={"Accept":"application/json","User-Agent":"Voucherbase/1.0 (property planning)"}
    if token: headers["Authorization"]="Bearer "+token
    for attempt in range(attempts):
        try:
            with build_opener(NoRedirect).open(Request(url,headers=headers),timeout=12) as r:
                raw=r.read(8_000_001)
                if len(raw)>8_000_000: raise ValueError("Provider response too large")
                return json.loads(raw),hashlib.sha256(raw).hexdigest()
        except HTTPError as e:
            if e.code not in (429,500,502,503,504) or attempt==attempts-1: raise
            time.sleep(min(2**attempt,4))

def geocode(address, provider="census"):
    if not isinstance(address,str) or not 3<=len(address)<=500: raise ValueError("Invalid address")
    if provider=="fixture":
        # No actual parcel is asserted by these fictional addresses.
        if address.strip().lower()!="100 example lane, denver, co 80204": return {"matches":[],"provider":"synthetic","status":"no_match"}
        return {"status":"matched","provider":"synthetic","matches":[{"address":address,"latitude":39.737,"longitude":-105.01,"state":"CO","zip":"80204","quality":"synthetic","warning":"Fictional address and coordinates for demonstration only"}]}
    if provider!="census": raise ValueError("Unknown geocoder")
    if os.getenv("VB_LIVE_PROVIDERS","0")!="1":
        return {"status":"provider_disabled","matches":[],"provider":"census","warning":"Enable VB_LIVE_PROVIDERS=1 to transmit the address to the US Census Geocoder"}
    url="https://geocoding.geo.census.gov/geocoder/locations/onelineaddress?"+urlencode({"address":address,"benchmark":"Public_AR_Current","format":"json"})
    raw,digest=fetch_json(url)
    matches=[]
    for m in raw.get("result",{}).get("addressMatches",[])[:10]:
        p=m.get("coordinates",{}); a=m.get("addressComponents",{})
        if not isinstance(p.get("x"),(int,float)) or not isinstance(p.get("y"),(int,float)): continue
        matches.append({"address":m.get("matchedAddress",address),"latitude":p["y"],"longitude":p["x"],"state":a.get("state",""),"zip":a.get("zip",""),"quality":"census_interpolated","warning":"Interpolated address-range match, not verified parcel coordinates"})
    return {"status":"matched" if len(matches)==1 else "ambiguous" if matches else "no_match","matches":matches,"provider":"census","response_sha256":digest,"benchmark":"Public_AR_Current"}

def money_cents(value):
    if isinstance(value,bool): raise ValueError("Boolean is not money")
    try: n=Decimal(str(value).replace(",","").replace("$",""))
    except InvalidOperation as e: raise ValueError("Invalid money") from e
    if not n.is_finite() or n<0 or n>Decimal("10000000000") or n*100 != (n*100).to_integral_value(): raise ValueError("Invalid amount/precision")
    return int(n*100)

def normalize_hud(payload, year, entity):
    data=payload.get("data",{})
    basic=data.get("basicdata")
    if not isinstance(basic,(dict,list)): raise ValueError("HUD basicdata absent: review provider schema")
    actual=data.get("year",basic.get("year") if isinstance(basic,dict) else None)
    if str(actual)!=str(year): raise ValueError("HUD fiscal year mismatch")
    fields=["Efficiency","One-Bedroom","Two-Bedroom","Three-Bedroom","Four-Bedroom"]
    rows=[]
    for row in basic if isinstance(basic,list) else [basic]:
        zip_code=row.get("zip_code","MSA level")
        if zip_code!="MSA level" and not re.fullmatch(r"\d{5}",zip_code): raise ValueError("Unexpected HUD geography")
        for bedroom, key in enumerate(fields):
            if key not in row: raise ValueError("Incomplete HUD bedroom schedule")
            rows.append({"kind":"fmr" if zip_code=="MSA level" else "safmr","geo_key":entity if zip_code=="MSA level" else zip_code,"bedrooms":bedroom,"structure":"any","utility_code":"none","value_cents":money_cents(row[key])})
    return rows

def hud_snapshot(entity, year):
    if not re.fullmatch(r"[A-Za-z0-9]{5,30}",entity) or not 2017<=year<=2100: raise ValueError("Invalid HUD entity/year")
    token=os.environ.get("HUD_API_TOKEN")
    if not token: raise ValueError("HUD_API_TOKEN is required; never embed it in source")
    url=f"https://www.huduser.gov/hudapi/public/fmr/data/{entity}?year={year}"
    data,checksum=fetch_json(url,token)
    return {"source_url":url,"raw_sha256":checksum,"payload":data,"rows":normalize_hud(data,year,entity)}
