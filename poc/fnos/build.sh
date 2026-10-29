#!/bin/bash
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
version="$(sed -n 's/^version=//p' "$here/manifest")"
fnpack="${FNPACK:-fnpack}"
out="$root/dist"
stage="$(mktemp -d "${TMPDIR:-/tmp}/qas-fpk.XXXXXX")"
trap 'rm -rf "$stage"' EXIT

mkdir -p "$stage/app/web" "$out"
(cd "$root/poc" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$stage/app/qas" .)
cp -R "$root/web/dist" "$stage/app/web/dist"
cp -R "$here/app/ui" "$stage/app/ui"
cp -R "$here/cmd" "$here/config" "$here/wizard" "$stage/"
chmod +x "$stage"/cmd/*
cp "$here/manifest" "$here/ICON.PNG" "$here/ICON_256.PNG" "$stage/"

(cd "$stage" && "$fnpack" build --directory .)
cp "$stage/qas.fpk" "$out/qas_${version}_x86.fpk"
echo "$out/qas_${version}_x86.fpk"
