#!/bin/sh
# Monta a release da Guarita como a de uma versão, sem publicar nada e
# sem rede: o contêiner roda com --network none, os módulos vêm do cache
# local, só para leitura, e as ferramentas, de $TOOLS.
#   TOOLS=dir-com-goreleaser-syft-e-cosign \
#   COSIGN_PRIVATE_KEY="$(cat cosign.key)" COSIGN_PASSWORD=… \
#   deploy/release/snapshot.sh [opções do goreleaser release]
set -eu
image=golang:1.27.2@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8
# #region release-snapshot
docker run --rm --network none -u "$(id -u):$(id -g)" -e HOME=/tmp \
  -v "$TOOLS:/tools:ro" -v "$(go env GOMODCACHE):/go/pkg/mod:ro" \
  -v "$PWD:/src" -w /src \
  -e GOFLAGS=-mod=readonly -e GOPROXY=off -e GOTOOLCHAIN=local \
  -e SYFT_CHECK_FOR_APP_UPDATE=false \
  -e COSIGN_PRIVATE_KEY -e COSIGN_PASSWORD \
  "$image" sh -c 'git config --global --add safe.directory /src &&
    PATH=/tools:$PATH goreleaser release --snapshot --clean "$@"' \
  sh "$@"
# #endregion
