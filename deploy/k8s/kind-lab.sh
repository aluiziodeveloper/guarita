#!/bin/sh
# Laboratório do Kubernetes num cluster local do kind: carrega as
# imagens, cria o Secret, aplica o manifesto, chama a Guarita de um pod
# de agente e de um pod sem a marca, lê as métricas e apaga tudo.
#   kind create cluster --name guarita && deploy/k8s/kind-lab.sh
set -eu
cd "$(dirname "$0")"
curl_image=curlimages/curl:8.16.0@sha256:463eaf6072688fe96ac64fa623fe73e1dbe25d8ad6c34404a669ad3ce1f104b6
token() { go run ../../cmd/lab-token "$@"; }
# #region deploy-k8s-lab
dir=$(mktemp -d) && chmod 700 "$dir"
token -genkey -out "$dir"
token -cert guarita.guarita.svc -out "$dir"
head -c 32 /dev/urandom | base64 > "$dir/audit.key"
kind load docker-image --name guarita guarita:cap21 guarita-lab:cap21 \
  >/dev/null 2>&1
kubectl apply -f guarita.yaml >/dev/null
kubectl -n guarita create secret generic guarita-secrets \
  --from-file="$dir/agent.pub" --from-file="$dir/tls.crt" \
  --from-file="$dir/tls.key" --from-file="$dir/audit.key" >/dev/null
kubectl -n guarita create configmap ca --from-file="$dir/tls.crt" \
  >/dev/null
kubectl -n guarita rollout status deploy/guarita --timeout=120s
url=https://guarita.guarita.svc:8443/mcp/files
agent=$(token -key "$dir/agent.key" -iss https://auth.lab.test \
  -aud "$url" -sub relatorios)
for role in agent outro; do
  code=$(sh ./call.sh "$role" "$curl_image" "$url" "$agent")
  echo "pod $role: $code"
done
# #endregion
kubectl -n guarita get pod -l app=guarita \
  -o jsonpath='{range .items[*].status.containerStatuses[*]}{.name} pronto={.ready} reinícios={.restartCount}{"\n"}{end}'
kubectl delete -f guarita.yaml --wait=true >/dev/null
rm -rf "$dir"
