#!/bin/bash
# c1q.sh "SQL"  — run a query against the 1C MSSQL using Gateway's saved connection
set -e
cd /home/gateway
ENC=$(sudo docker exec gateway-postgres-1 psql -U gateway -d tenant_test -At -c "SELECT config_enc FROM connections WHERE connector_type='mssql' LIMIT 1")
KEY=$(grep ^ENCRYPTION_KEY .env | cut -d= -f2 | tr -d '\r"')
python3 -c "import cryptography" 2>/dev/null || pip install -q --break-system-packages cryptography >/dev/null 2>&1 || pip install -q cryptography >/dev/null 2>&1
CFG=$(python3 - "$ENC" "$KEY" <<'PY'
import sys,base64,json
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
raw=base64.b64decode(sys.argv[1]); k=bytes.fromhex(sys.argv[2])
c=json.loads(AESGCM(k).decrypt(raw[:12],raw[12:],None))
print(c['host']+','+(c.get('port') or '1433')); print(c['database']); print(c['username']); print(c['password'])
PY
)
H=$(sed -n 1p <<<"$CFG"); D=$(sed -n 2p <<<"$CFG"); U=$(sed -n 3p <<<"$CFG"); P=$(sed -n 4p <<<"$CFG")
echo "== 1C: host=$H db=$D user=$U ==" >&2
# query: from file if $1 is a readable file, else the literal text, else stdin
if [ -n "$1" ] && [ -f "$1" ]; then cp "$1" /tmp/c1q.sql; elif [ -n "$1" ]; then printf '%s\n' "$1" > /tmp/c1q.sql; else cat > /tmp/c1q.sql; fi
sudo docker run --rm -v /tmp/c1q.sql:/q.sql:ro mcr.microsoft.com/mssql-tools /opt/mssql-tools/bin/sqlcmd -S "$H" -d "$D" -U "$U" -P "$P" -C -y 0 -s'|' -i /q.sql
