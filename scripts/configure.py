#!/usr/bin/env python3
"""VOUCHERBASE | Parrish Lyon | PL-VOUCHERBASE-20260914. Generate local credentials; never commit .env."""
import os,secrets
from pathlib import Path
root=Path(__file__).resolve().parents[1]
path=root/".env"
if path.exists():
    print("Existing .env kept unchanged.")
else:
    keys=["VB_DB_ADMIN_PASSWORD","VB_DB_APP_PASSWORD","VB_DB_INGEST_PASSWORD","VB_INTERNAL_TOKEN","VB_API_TOKEN"]
    content="\n".join(k+"="+secrets.token_urlsafe(36) for k in keys)+"\nVB_DEMO=1\nVB_LIVE_PROVIDERS=0\nVB_PUBLIC_ORIGIN=http://localhost:8080\nHUD_API_TOKEN=\n"
    content += f"VB_LOCAL_UID={os.getuid()}\nVB_LOCAL_GID={os.getgid()}\n"
    fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
    with os.fdopen(fd,"w") as f:f.write(content)
    print("Created .env with unique credentials (permissions 0600).")
(root/"imports").mkdir(exist_ok=True)
print("Start: docker compose up --build -d. Read VB_API_TOKEN locally from .env to sign in.")
