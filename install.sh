#!/bin/sh
# Install a prebuilt netflix binary from GitHub Releases.
set -eu

REPO="seifreed/netflixcli"
BIN="netflix"

info() { printf '%s\n' "==> $1" >&2; }
err() {
	printf '%s\n' "error: $1" >&2
	exit 1
}

command -v curl >/dev/null 2>&1 || err "curl is required"
command -v tar >/dev/null 2>&1 || err "tar is required"

os=$(uname -s)
case "$os" in
Darwin) os=darwin ;;
Linux) os=linux ;;
*) err "unsupported OS '$os'" ;;
esac

arch=$(uname -m)
case "$arch" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) err "unsupported architecture '$arch'" ;;
esac

tag="${NETFLIX_VERSION:-}"
if [ -z "$tag" ]; then
	info "resolving latest release..."
	tag=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
		grep '"tag_name"' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
	[ -n "$tag" ] || err "could not determine latest release"
fi
case "$tag" in
v*) version=${tag#v} ;;
*) version=$tag; tag=v$tag ;;
esac

asset="${BIN}_${version}_${os}_${arch}.tar.gz"
base="https://github.com/${REPO}/releases/download/${tag}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

info "downloading ${asset}"
curl -fsSL "${base}/${asset}" -o "${tmp}/${asset}" || err "download failed"
curl -fsSL "${base}/checksums.txt" -o "${tmp}/checksums.txt" ||
	err "could not fetch checksums.txt; refusing to install unverified binary"

(
	cd "$tmp"
	grep " ${asset}\$" checksums.txt >expected.txt || err "no checksum for ${asset}"
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum -c expected.txt >/dev/null || err "checksum mismatch"
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 -c expected.txt >/dev/null || err "checksum mismatch"
	else
		err "sha256sum or shasum is required"
	fi
)

tar -xzf "${tmp}/${asset}" -C "$tmp" || err "failed to extract archive"
[ -f "${tmp}/${BIN}" ] || err "binary not found in archive"
chmod +x "${tmp}/${BIN}"

dir="${NETFLIX_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
	if [ -w /usr/local/bin ] 2>/dev/null; then
		dir=/usr/local/bin
	else
		dir="${HOME}/.local/bin"
	fi
fi
mkdir -p "$dir" || err "could not create install directory '$dir'"

if mv "${tmp}/${BIN}" "${dir}/${BIN}" 2>/dev/null; then
	:
elif command -v sudo >/dev/null 2>&1; then
	info "writing to ${dir} needs sudo..."
	sudo mv "${tmp}/${BIN}" "${dir}/${BIN}" || err "install failed"
else
	err "cannot write to ${dir}; set NETFLIX_INSTALL_DIR"
fi

info "installed ${BIN} to ${dir}/${BIN}"
