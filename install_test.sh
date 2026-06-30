#!/bin/sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
INSTALL_SH="$ROOT_DIR/install.sh"

fail() {
  printf 'not ok - %s\n' "$1" >&2
  exit 1
}

assert_contains() {
  haystack=$1
  needle=$2
  name=$3
  case "$haystack" in
    *"$needle"*) printf 'ok - %s\n' "$name" ;;
    *) fail "$name: expected output to contain $needle; got: $haystack" ;;
  esac
}

run_dry() {
  AGW_INSTALL_DRY_RUN=1 AGW_TEST_OS=$1 AGW_TEST_ARCH=$2 AGW_VERSION=v0.1.0 sh "$INSTALL_SH"
}

output=$(run_dry Darwin arm64)
assert_contains "$output" "agw_Darwin_arm64.tar.gz" "maps Darwin arm64 artifact"
assert_contains "$output" "v0.1.0" "uses pinned version in dry run"

output=$(run_dry Linux x86_64)
assert_contains "$output" "agw_Linux_x86_64.tar.gz" "maps Linux x86_64 artifact"

if AGW_INSTALL_DRY_RUN=1 AGW_TEST_OS=FreeBSD AGW_TEST_ARCH=amd64 sh "$INSTALL_SH" >/tmp/agw-install-test.out 2>&1; then
  fail "unsupported OS should fail"
fi
assert_contains "$(cat /tmp/agw-install-test.out)" "unsupported operating system" "rejects unsupported OS"
rm -f /tmp/agw-install-test.out

tmpdir=$(mktemp -d)
output=$(AGW_INSTALL_DRY_RUN=1 AGW_TEST_OS=Linux AGW_TEST_ARCH=amd64 AGW_VERSION=v0.1.0 AGW_INSTALL_DIR="$tmpdir/bin" sh "$INSTALL_SH")
assert_contains "$output" "install_dir=$tmpdir/bin" "uses AGW_INSTALL_DIR override"
rmdir "$tmpdir"
