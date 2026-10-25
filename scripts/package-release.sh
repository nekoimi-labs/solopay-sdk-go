#!/usr/bin/env bash
set -euo pipefail

tag="${1:?usage: scripts/package-release.sh v0.2.0}"
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "release tag must match v1.2.3 or v1.2.3-rc.1" >&2
  exit 1
fi

version="${tag#v}"
grep -q "Version = \"$version\"" version.go || {
  echo "version.go does not contain SDK version $version" >&2
  exit 1
}
git show-ref --verify --quiet "refs/tags/$tag" || {
  echo "tag $tag is not available locally" >&2
  exit 1
}
if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "working tree must be clean before packaging" >&2
  exit 1
fi

mkdir -p dist
archive="dist/solopay-sdk-go-$tag.tar.gz"
git archive --format=tar.gz --prefix="solopay-sdk-go-$tag/" "$tag" -o "$archive"
(cd dist && sha256sum "solopay-sdk-go-$tag.tar.gz" > "solopay-sdk-go-$tag.SHA256SUMS")
printf '%s\n' "$archive" "dist/solopay-sdk-go-$tag.SHA256SUMS"
