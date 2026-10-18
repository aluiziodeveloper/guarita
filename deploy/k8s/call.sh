#!/bin/sh
# Um pod de curl, com a marca dada, chama a Guarita com o token e diz o
# código HTTP; sem resposta em 5 s, diz que não chegou.
#   call.sh papel imagem url token
role=$1 image=$2 url=$3 token=$4
body='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"read_file"}}'
code=$(kubectl -n guarita run "curl-$role" --rm -i --restart=Never --quiet \
  --labels="role=$role" --image="$image" --overrides='{"spec":{
    "volumes":[{"name":"ca","configMap":{"name":"ca"}}],
    "containers":[{"name":"c","image":"'"$image"'",
      "volumeMounts":[{"name":"ca","mountPath":"/ca"}],
      "args":["-s","-o","/dev/null","-w","%{http_code}","-m","5",
        "--cacert","/ca/tls.crt","-H","Authorization: Bearer '"$token"'",
        "-d","'"$(printf '%s' "$body" | sed 's/"/\\"/g')"'","'"$url"'"]}]}}' \
  2>/dev/null)
case "$code" in
  ''|000) echo "sem resposta em 5 s" ;;
  *) echo "$code" ;;
esac
