#!/bin/sh
# Laboratório do Compose de produção: gera os segredos, sobe, chama a
# Guarita com e sem token, lê as métricas e a trilha, e derruba tudo.
#   deploy/prod/lab.sh
set -eu
cd "$(dirname "$0")"
token() { go run ../../cmd/lab-token "$@"; }
# #region deploy-prod-lab
mkdir -p secrets && chmod 700 secrets
token -genkey -out secrets
token -cert guarita.lab -out secrets
head -c 32 /dev/urandom | base64 > secrets/audit.key
chmod 600 secrets/audit.key
docker compose up -d --build --wait >/dev/null 2>&1
url=https://guarita.lab:8443/mcp/files
call() { # $1: token; $2: ferramenta
  curl -s -o /dev/null -w "%{http_code} " --cacert secrets/tls.crt \
    --resolve guarita.lab:8443:127.0.0.1 -H "Authorization: Bearer $1" \
    -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":'"\
{\"name\":\"$2\"}}" "$url"
}
agent=$(token -key secrets/agent.key -iss https://auth.lab.test \
  -aud "$url" -sub relatorios)
echo "sem token: $(call '' read_file)"
echo "read_file: $(call "$agent" read_file)"
echo "delete_file: $(call "$agent" delete_file)"
curl -s http://127.0.0.1:9090/metrics | grep 'messages_total{'
# #endregion
docker compose ps --format '{{.Service}} {{.Status}}'
# A trilha, verificada com a própria imagem, no volume dela.
docker run --rm -v guarita-prod_guarita-audit:/a:ro guarita:cap21 \
  audit verify /a/auditoria.jsonl
docker inspect guarita-prod-guarita-1 --format \
  'ro={{.HostConfig.ReadonlyRootfs}} user={{.Config.User}} caps={{.HostConfig.CapDrop}}'
docker compose down -v >/dev/null 2>&1
rm -rf secrets
