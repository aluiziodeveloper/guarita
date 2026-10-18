#!/bin/sh
# Laboratório do Capítulo 20: três adulterações numa cópia da release
# local, conferidas por verify.sh. Nada é publicado; a cópia é apagada.
#   TOOLS=… deploy/release/tamper.sh dist cosign.pub outra.pub
set -u
# #region release-tamper
dist=$1 key=$2 other=$3
copy() { rm -rf "$work" && cp -r "$dist" "$work"; }
check() { # o status de verify.sh, e só as linhas que não são OK
  sh deploy/release/verify.sh "$work" "$1" > "$work.out" 2>&1
  status=$?
  grep -v ': OK$' "$work.out"
  echo "status: $status"
}
work=$(mktemp -d)/dist
pkg=$(cd "$dist" && ls ./*_linux_amd64.tar.gz)
echo "== pacote trocado: um byte a mais em $pkg"
copy && printf '[LAB]' >> "$work/$pkg"
check "$key"
echo "== pacote trocado e soma refeita em checksums.txt"
copy && printf '[LAB]' >> "$work/$pkg"
sum=$(sha256sum "$work/$pkg" | cut -d' ' -f1)
sed -i "s|^[0-9a-f]*  ${pkg#./}\$|$sum  ${pkg#./}|" "$work/checksums.txt"
check "$key"
echo "== release intacta, chave pública de outra pessoa"
copy && check "$other"
rm -rf "$(dirname "$work")"
# #endregion
