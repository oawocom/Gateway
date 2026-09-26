#!/bin/bash
# c1ver.sh — 1C platform / configuration version + DBNames sample (read-only)
set -e
cat > /tmp/c1ver.sql <<'SQL'
SET NOCOUNT ON;
SELECT 'IBVCOL' k, c.name v FROM sys.columns c WHERE c.object_id=OBJECT_ID('IBVersion');
SELECT 'PARAMS' k, FileName v FROM Params;
SELECT 'CFGFILES' k, CAST(COUNT(*) AS varchar) v FROM Config;
SELECT 'DBNAMES' k, CONVERT(varchar(max), BinaryData, 1) v FROM Params WHERE FileName='DBNames';
SELECT 'ROOT' k, CONVERT(varchar(max), BinaryData, 1) v FROM Config WHERE FileName='root';
SQL
/home/gateway/c1q.sh /tmp/c1ver.sql 2>&1 | python3 -c '
import sys,zlib,re
for line in sys.stdin:
    if "|" not in line or line.startswith(("k|","-|","==")): continue
    k,v=line.rstrip("\n").split("|",1); v=v.strip()
    if k in("DBNAMES","ROOT") and v.startswith("0x"):
        raw=bytes.fromhex(v[2:])
        try: txt=zlib.decompress(raw,-15).decode("utf-8","replace")
        except Exception as e: txt="(decompress failed: %s)"%e
        if k=="ROOT": print("ROOT:",txt[:300])
        else:
            print("DBNAMES entries:",txt.count("\n"))
            for m in re.finditer(r"\{\"?([0-9a-f-]{36})\"?,\"(Reference|Document|AccRg|Acc|AccRgED)\",([0-9]+)\}",txt): pass
            for l in txt.splitlines():
                if any(s in l for s in ("Reference\",62}","Reference\",48}","Reference\",72}","Document\",209}","Document\",192}","AccRg\",517}","Acc\",19}")): print("  ",l.strip())
    else: print(k+":",v)
'
