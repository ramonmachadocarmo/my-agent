#!/usr/bin/env bash
# Build my-agent into ./bin. Usage: scripts/build.sh [test|build|all]
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION="$(sed -n 's/^## \[\([0-9][^]]*\)\].*/\1/p' CHANGELOG.md | head -n1)"
VERSION="${VERSION:-dev}"
EXT=""; case "$(uname -s)" in MINGW*|MSYS*|CYGWIN*) EXT=".exe";; esac

do_test()  { go vet ./... && go test ./...; }
do_build() {
  mkdir -p bin
  go build -trimpath -ldflags "-s -w -X my-agent/internal/cli.Version=${VERSION}" -o "bin/my-agent${EXT}" ./cmd/my-agent
  echo "built bin/my-agent${EXT} (${VERSION})"
}

case "${1:-all}" in
  test)  do_test ;;
  build) do_build ;;
  all)   do_test && do_build ;;
  *) echo "usage: $0 [test|build|all]" >&2; exit 2 ;;
esac
