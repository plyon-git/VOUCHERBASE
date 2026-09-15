"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914."""
import hmac, json, os, socket, threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from .model import compare
from .providers import geocode
TOKEN=os.getenv("VB_INTERNAL_TOKEN","")
LIMIT=threading.BoundedSemaphore(8)
class Handler(BaseHTTPRequestHandler):
    def setup(self):
        super().setup(); self.connection.settimeout(20)
    def log_message(self,*args): pass  # do not record property addresses or credentials
    def send_json(self,status,data):
        raw=json.dumps(data,allow_nan=False).encode();self.send_response(status)
        self.send_header("Content-Type","application/json");self.send_header("Content-Length",str(len(raw)));self.end_headers();self.wfile.write(raw)
    def do_GET(self):
        self.send_json(200,{"status":"ok","version":"pipeline/1.0.0"}) if self.path=="/healthz" else self.send_json(404,{"error":"not_found"})
    def do_POST(self):
        if len(TOKEN)<32 or not hmac.compare_digest(self.headers.get("X-Internal-Token",""),TOKEN):return self.send_json(401,{"error":"unauthorized"})
        if not LIMIT.acquire(blocking=False):return self.send_json(429,{"error":"busy"})
        try:
            size=int(self.headers.get("Content-Length","0"))
            if not 0<size<=2_000_000:return self.send_json(413,{"error":"body_size"})
            req=json.loads(self.rfile.read(size))
            if self.path=="/v1/geocode":out=geocode(req["address"],req.get("provider","census"))
            elif self.path=="/v1/comparables":out=compare(req["subject"],req["candidates"],req["effective_on"],req["knowledge_at"],req.get("kind","executed"))
            else:return self.send_json(404,{"error":"not_found"})
            self.send_json(200,out)
        except (ValueError,TypeError,KeyError):self.send_json(422,{"error":"invalid_input_or_provider_schema"})
        except Exception:self.send_json(502,{"error":"provider_unavailable"})
        finally:LIMIT.release()
if __name__=="__main__":
    if len(TOKEN)<32:raise SystemExit("VB_INTERNAL_TOKEN must contain at least 32 characters")
    ThreadingHTTPServer(("0.0.0.0",8082),Handler).serve_forever()
