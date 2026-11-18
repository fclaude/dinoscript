#!/usr/bin/env bash
set -euo pipefail

tag="$GITHUB_REF_NAME"
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ || "$(git rev-parse HEAD)" != "$GITHUB_SHA" ]]; then
  echo "The release requires a version tag and its checked-out commit." >&2
  exit 1
fi

staging="$(mktemp -d)"
trap 'rm -rf -- "$staging"' EXIT

mac=build/release-artifacts/Dinoscript-mac-development
for arch in arm64 amd64; do
  image="$(find "$mac" -type f -name "Dinoscript-$arch.dmg")"
  test -f "$image" && test -s "$image"
  cp "$image" "$staging/"
done

windows=build/release-artifacts/Dinoscript-windows-x64-development
book=Dinoscript-with-Pip.pdf
cmp "$windows/$book" "book/$book"
cp "book/$book" "$staging/"
(
  cd "$windows"
  for file in Dinoscript.exe "$book" LICENSE NOTICE THIRD-PARTY-NOTICES.txt; do
    test -s "$file"
  done
  zip -q "$staging/Dinoscript-Windows-x64.zip" Dinoscript.exe "$book" LICENSE NOTICE THIRD-PARTY-NOTICES.txt
)
(
  cd "$staging"
  sha256sum Dinoscript-arm64.dmg Dinoscript-amd64.dmg Dinoscript-Windows-x64.zip "$book" > SHA256SUMS
)

repo="$GITHUB_REPOSITORY"
version="${tag#v}"
cat > "$staging/notes.md" <<'NOTES'
Dinoscript is an offline coding editor for kids. Open the editor and press F5 to run a program. Press F1 to learn with Pip the triceratops.

- On Apple silicon, use the arm64 disk image. On Intel Macs, use the amd64 disk image. Drag Dinoscript to Applications.
- On Windows x64, extract the ZIP and open Dinoscript.exe.
- [Read Dinoscript with Pip online](https://dinoscript.org/workbook.html) or download the printable PDF below.

The Mac apps are ad-hoc signed. The Windows executable is unsigned.
NOTES

gh release create "$tag" --repo "$repo" --verify-tag --draft \
  --title "Dinoscript $version" --notes-file "$staging/notes.md" \
  "$staging/Dinoscript-arm64.dmg" "$staging/Dinoscript-amd64.dmg" \
  "$staging/Dinoscript-Windows-x64.zip" "$staging/$book" "$staging/SHA256SUMS"

id="$(gh api "repos/$repo/releases" --paginate --slurp |
  jq -er --arg tag "$tag" 'flatten | map(select(.tag_name == $tag and .draft)) | .[0].id')"
gh api "repos/$repo/releases/$id/assets" > "$staging/remote-assets.json"
(
  cd "$staging"
  sha256sum SHA256SUMS > expected-sha
  cat SHA256SUMS >> expected-sha
  jq -r '.[] | select(.state == "uploaded") | "\(.digest | sub("^sha256:"; ""))  \(.name)"' remote-assets.json |
    sort > remote-sha
  sort expected-sha | diff -u - remote-sha
)
gh release edit "$tag" --repo "$repo" --draft=false
gh release view "$tag" --repo "$repo" --json url,isDraft --jq 'select(.isDraft | not) | .url'
