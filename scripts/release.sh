#!/usr/bin/env bash
# Build release packages for every supported platform into dist/:
# plain macOS and Windows binaries, Linux AppImage and tar.gz, plus
# SHA256SUMS.txt.
#
#   scripts/release.sh <tag> [source-dir]
#
# source-dir is the checkout to build (default: this repository), so an older
# tag can be packaged with the current packaging files. AppImages are built
# with appimagetool, which needs a Linux host (x86_64 or aarch64; CI or Docker).
set -euo pipefail

TAG=${1:?usage: scripts/release.sh <tag, e.g. v0.1.7> [source-dir]}
HERE=$(cd "$(dirname "$0")/.." && pwd)
SRC=$(cd "${2:-$HERE}" && pwd)
DIST=${DIST:-$HERE/dist}
TOOLS=$DIST/.tools
for tool in go tar curl file; do
  command -v "$tool" >/dev/null || { echo "release.sh needs '$tool' on PATH" >&2; exit 1; }
done
mkdir -p "$DIST" "$TOOLS"
rm -f "$DIST"/learn-* "$DIST"/SHA256SUMS.txt

# The binary must report the version being released.
got=$(cd "$SRC" && CGO_ENABLED=0 go run ./cmd/learn version)
[ "$got" = "learn $TAG" ] || { echo "version mismatch: tag $TAG, binary says '$got'" >&2; exit 1; }

build() { # goos goarch output [goarm]
  (cd "$SRC" && CGO_ENABLED=0 GOOS=$1 GOARCH=$2 GOARM=${4:-} go build -trimpath -ldflags="-s -w" -o "$3" ./cmd/learn)
}

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# macOS and Windows get the plain binary: an archive would only hold that
# one file. macOS: amd64 = x64, arm64 = Apple silicon.
for pair in amd64:x64 arm64:arm64; do
  build darwin "${pair%%:*}" "$DIST/learn-$TAG-macos-${pair##*:}"
done

# Windows: 386 = x86, amd64 = x64, arm64.
for pair in 386:x86 amd64:x64 arm64:arm64; do
  build windows "${pair%%:*}" "$DIST/learn-$TAG-windows-${pair##*:}.exe"
done

# Linux: AppImages. goarch:goarm:appimage-arch:name
fetch() { [ -s "$2" ] || curl -fsSL --retry 3 -o "$2" "$1"; }
host=$(uname -m)
case $host in x86_64 | aarch64) ;; arm64) host=aarch64 ;; *) echo "appimagetool needs an x86_64 or aarch64 Linux host, not $host" >&2; exit 1 ;; esac
fetch "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-$host.AppImage" "$TOOLS/appimagetool"
chmod +x "$TOOLS/appimagetool"
for spec in 386::i686:x86 amd64::x86_64:x64 arm64::aarch64:arm64 arm:7:armhf:armv7; do
  IFS=: read -r arch goarm apparch name <<<"$spec"
  fetch "https://github.com/AppImage/type2-runtime/releases/download/continuous/runtime-$apparch" "$TOOLS/runtime-$apparch"
  appdir=$work/linux-$name/learn.AppDir
  mkdir -p "$appdir/usr/bin"
  build linux "$arch" "$appdir/usr/bin/learn" "$goarm"
  cp "$HERE/packaging/linux/AppRun" "$appdir/AppRun"
  chmod +x "$appdir/AppRun"
  cp "$HERE/packaging/linux/learn.desktop" "$HERE/packaging/linux/learn.svg" "$appdir/"
  if ! log=$(ARCH=$apparch APPIMAGE_EXTRACT_AND_RUN=1 "$TOOLS/appimagetool" --no-appstream \
    --runtime-file "$TOOLS/runtime-$apparch" "$appdir" "$DIST/learn-$TAG-linux-$name.AppImage" 2>&1); then
    echo "$log" >&2
    exit 1
  fi
  # Plain binary for servers and containers without FUSE.
  tar -czf "$DIST/learn-$TAG-linux-$name.tar.gz" -C "$appdir/usr/bin" learn
done

if command -v sha256sum >/dev/null; then sum=(sha256sum); else sum=(shasum -a 256); fi
(cd "$DIST" && "${sum[@]}" learn-* >SHA256SUMS.txt)
ls -1 "$DIST"
