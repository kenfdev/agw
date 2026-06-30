#!/bin/sh
set -eu

REPO=${AGW_REPO:-kenfdev/agw}
VERSION=${AGW_VERSION:-}
OS=${AGW_TEST_OS:-$(uname -s)}
ARCH=${AGW_TEST_ARCH:-$(uname -m)}

case "$OS" in
  Darwin|Linux) ;;
  *) echo "unsupported operating system: $OS" >&2; exit 1 ;;
esac

case "$ARCH" in
  arm64|aarch64) ARTIFACT_ARCH=arm64 ;;
  x86_64|amd64) ARTIFACT_ARCH=x86_64 ;;
  *) echo "unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

if [ -n "${AGW_INSTALL_DIR:-}" ]; then
  INSTALL_DIR=$AGW_INSTALL_DIR
elif [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
  INSTALL_DIR=/usr/local/bin
else
  INSTALL_DIR=$HOME/.local/bin
fi

if [ -z "$VERSION" ]; then
  VERSION=latest
fi

ARCHIVE="agw_${OS}_${ARTIFACT_ARCH}.tar.gz"

if [ "${AGW_INSTALL_DRY_RUN:-0}" = "1" ]; then
  echo "repo=$REPO"
  echo "version=$VERSION"
  echo "archive=$ARCHIVE"
  echo "install_dir=$INSTALL_DIR"
  exit 0
fi

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "required command not found: $1" >&2
    exit 1
  fi
}

need curl
need tar

if command -v sha256sum >/dev/null 2>&1; then
  SHA256_CMD=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  SHA256_CMD="shasum -a 256"
else
  echo "required command not found: sha256sum or shasum" >&2
  exit 1
fi

API_URL="https://api.github.com/repos/$REPO/releases/latest"
if [ "$VERSION" = "latest" ]; then
  VERSION=$(curl -fsSL "$API_URL" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
fi

if [ -z "$VERSION" ]; then
  echo "failed to resolve latest AGW version" >&2
  exit 1
fi

BASE_URL="https://github.com/$REPO/releases/download/$VERSION"
TMP_DIR=$(mktemp -d)
cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

curl -fsSLo "$TMP_DIR/$ARCHIVE" "$BASE_URL/$ARCHIVE"
curl -fsSLo "$TMP_DIR/checksums.txt" "$BASE_URL/checksums.txt"

expected=$(grep "  $ARCHIVE\$" "$TMP_DIR/checksums.txt" | awk '{print $1}')
if [ -z "$expected" ]; then
  echo "checksum for $ARCHIVE not found" >&2
  exit 1
fi

actual=$($SHA256_CMD "$TMP_DIR/$ARCHIVE" | awk '{print $1}')
if [ "$actual" != "$expected" ]; then
  echo "checksum mismatch for $ARCHIVE" >&2
  exit 1
fi

tar -xzf "$TMP_DIR/$ARCHIVE" -C "$TMP_DIR" agw
mkdir -p "$INSTALL_DIR"
mv "$TMP_DIR/agw" "$INSTALL_DIR/agw"
chmod 0755 "$INSTALL_DIR/agw"

printf 'agw %s installed to %s/agw\n' "$VERSION" "$INSTALL_DIR"
"$INSTALL_DIR/agw" --version
