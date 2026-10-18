#!/bin/sh
# Confere uma release antes de usá-la: a assinatura de checksums.txt,
# com a chave pública de quem publicou, e a soma de cada pacote. No
# laboratório, o cosign roda sem rede, e a assinatura não está num log
# de transparência: por isso --insecure-ignore-tlog, que uma release
# pública, registrada no Rekor, não precisa.
#   TOOLS=dir-com-cosign deploy/release/verify.sh dist cosign.pub
set -eu
image=golang:1.27.2@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8
# #region release-verify
dist=$1 key=$2
docker run --rm --network none -u "$(id -u):$(id -g)" -e HOME=/tmp \
  -v "$TOOLS:/tools:ro" -v "$(realpath "$dist"):/dist:ro" \
  -v "$(realpath "$key"):/key.pub:ro" -w /dist "$image" sh -c '
    # Sem pipeline: num "a | b", o status é o de b, e uma assinatura
    # inválida passaria. Sem a assinatura, as somas nem são lidas.
    /tools/cosign verify-blob --key /key.pub --insecure-ignore-tlog \
      --bundle checksums.txt.sigstore.json checksums.txt \
      2>/tmp/cosign.err || { grep -v WARNING /tmp/cosign.err; exit 1; }
    grep -v WARNING /tmp/cosign.err
    sha256sum -c checksums.txt'
# #endregion
