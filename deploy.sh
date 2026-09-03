#!/bin/bash
# Gateway full deploy — run from /home/gateway after extracting the archive there
set -e
cd /home/gateway

echo "== 1/5 Backend fayllari kopyalanir =="
rm -rf backend
cp -r gateway-release/backend backend

echo "== 2/5 ENCRYPTION_KEY yoxlanilir =="
if ! grep -q ENCRYPTION_KEY .env; then
  echo "ENCRYPTION_KEY=$(openssl rand -hex 32)" >> .env
  echo "  yeni ENCRYPTION_KEY yaradildi"
fi
if ! grep -q ENCRYPTION_KEY docker-compose.yml; then
  sed -i '/JWT_SECRET:/a\      ENCRYPTION_KEY: ${ENCRYPTION_KEY}' docker-compose.yml
  echo "  docker-compose.yml yenilendi"
fi

echo "== 3/5 Backend build olunur =="
docker compose up -d --build

echo "== 4/5 Frontend build olunur =="
rm -rf frontend-src
cp -r gateway-release/frontend-src frontend-src
cd frontend-src
npm install --silent
npm run build
rm -rf /home/gateway/frontend/*
cp -r dist/* /home/gateway/frontend/
cd /home/gateway

echo "== 5/5 Yoxlama =="
sleep 3
docker compose ps
curl -s https://gateway.oawo.com/api/v1/health && echo
echo "HAZIR ✅"
