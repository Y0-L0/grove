#!/bin/sh
# Smoke-test an installed gw: check its version, then create, list and
# delete a workspace in a throwaway HOME.
#
#   sh scripts/smoke-test.sh [expected-version]
#
# GW names the binary (default: gw on PATH). The expected version may carry
# a leading v, as release tags do.
set -eu

gw=$(command -v "${GW:-gw}") || { echo "gw not found" >&2; exit 1; }
expected="${1:-}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
export HOME="$tmp/home" GIT_CONFIG_NOSYSTEM=1
mkdir -p "$HOME" "$tmp/repos/demo"

git config --global user.name smoke
git config --global user.email smoke@example.invalid
git config --global init.defaultBranch main
git -C "$tmp/repos/demo" init -q
git -C "$tmp/repos/demo" commit -q --allow-empty -m init

version=$("$gw" --version)
echo "$version"
if [ -n "$expected" ] && [ "$version" != "gw ${expected#v}" ]; then
  echo "expected gw ${expected#v}" >&2
  exit 1
fi

"$gw" init "$tmp/repos"
"$gw" create smoke -r demo -b smoke
"$gw" list
"$gw" delete smoke --force
