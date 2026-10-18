#!/bin/sh
# Confere se a release é reproduzível: clona o commit atual em dois
# caminhos diferentes, monta cada um com snapshot.sh, sem assinar, e
# compara as somas. Os binários e os pacotes precisam sair iguais.
#   TOOLS=dir-com-goreleaser-e-syft deploy/release/reproduce.sh
set -eu
# #region release-reproduce
work=$(mktemp -d)
commit=$(git rev-parse HEAD)
for dir in "$work/a/guarita" "$work/b/outro/caminho/guarita"; do
  git clone -q "$PWD" "$dir"
  git -C "$dir" checkout -q "$commit"
  (cd "$dir" && sh deploy/release/snapshot.sh --skip=sign >/dev/null)
done
echo "commit $commit"
cd "$work"
diff a/guarita/dist/checksums.txt \
  b/outro/caminho/guarita/dist/checksums.txt || true
grep -v sbom a/guarita/dist/checksums.txt > a.sums
grep -v sbom b/outro/caminho/guarita/dist/checksums.txt > b.sums
cmp a.sums b.sums && echo "binários e pacotes iguais: $(wc -l < a.sums)"
# #endregion
