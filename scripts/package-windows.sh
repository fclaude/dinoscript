#!/bin/sh
set -eu
cd "$(dirname "$0")/.."

output="${1:-build/windows-amd64}"
mkdir -p "$output"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath \
  -ldflags=-H=windowsgui -o "$output/Dinoscript.exe" ./cmd/dinoscript
cp book/Dinoscript-with-Pip.pdf book/Dinoscript-con-Pip.pdf LICENSE NOTICE release/THIRD-PARTY-NOTICES.txt "$output/"
