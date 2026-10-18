#!/bin/sh
# O mesmo commit em dois caminhos, com o go build comum: sem -trimpath,
# o caminho de cada cópia vai para o binário; com ele, não.
#   deploy/release/trimpath.sh
set -eu
# #region release-trimpath
work=$(mktemp -d)
for dir in a/guarita b/outro/caminho/guarita; do
  git clone -q "$PWD" "$work/$dir"
  git -C "$work/$dir" checkout -q "$(git rev-parse HEAD)"
done
cd "$work"
for flags in "" "-trimpath"; do
  for dir in a/guarita b/outro/caminho/guarita; do
    # O binário fica fora da cópia, para ela continuar sem mudanças.
    (cd "$dir" && CGO_ENABLED=0 go build $flags -o ../guarita.bin \
      ./cmd/guarita)
    printf '%-10s %-28s %s\n' "${flags:-sem}" "$dir" \
      "$(sha256sum "$(dirname "$dir")/guarita.bin" | cut -c1-16)"
  done
done
# #endregion
