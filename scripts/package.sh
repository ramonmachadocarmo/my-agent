#!/usr/bin/env bash
# Builds release archives for Windows, Linux and macOS into ./dist, each with
# the binary and its installer. Usage: scripts/package.sh [os/arch ...]
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="$(sed -n 's/^## \[\([0-9][^]]*\)\].*/\1/p' CHANGELOG.md | head -n1)"
VERSION="${VERSION:-dev}"
TARGETS=("$@")
[ ${#TARGETS[@]} -gt 0 ] || TARGETS=(windows/amd64 windows/arm64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64)

[ "${SKIP_TESTS:-0}" = 1 ] || { go vet ./... && go test ./...; }

rm -rf dist && mkdir -p dist
for t in "${TARGETS[@]}"; do
  os=${t%/*}; arch=${t#*/}
  name="my-agent-${VERSION}-${os}-${arch}"
  dir="dist/$name"
  mkdir -p "$dir"
  exe=my-agent; [ "$os" = windows ] && exe=my-agent.exe
  GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w -X my-agent/internal/cli.Version=${VERSION}" -o "$dir/$exe" ./cmd/my-agent
  cp README.md README.pt-BR.md README.es.md CHANGELOG.md "$dir/"
  cp -R docs "$dir/docs"
  if [ "$os" = windows ]; then
    cp scripts/install.ps1 scripts/install.cmd "$dir/"
    if command -v zip >/dev/null 2>&1; then
      (cd dist && zip -qr "$name.zip" "$name")
    else
      (cd dist && python3 -c 'import shutil,sys; shutil.make_archive(sys.argv[1], "zip", ".", sys.argv[1])' "$name")
    fi
  else
    cp scripts/install.sh "$dir/"
    chmod +x "$dir/install.sh" "$dir/$exe"
    tar -czf "dist/$name.tar.gz" -C dist "$name"
  fi
  rm -rf "$dir"
  echo "built $name"
done

(cd dist && { command -v sha256sum >/dev/null && sha256sum ./* || shasum -a 256 ./*; } | sed 's#\./##' > ../SHA256SUMS.tmp) && mv SHA256SUMS.tmp dist/SHA256SUMS
echo "archives in dist/"
