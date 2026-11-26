#!/bin/sh
# Prepare an isolated connector checkout; never modify an existing checkout.
set -eu

if [ "$#" -ne 1 ]; then
  echo "Usage: sh scripts/prepare-siwc-connector.sh NEW_DIRECTORY" >&2
  exit 2
fi
patch_dir=$(CDPATH= cd -- "$(dirname -- "$0")/../patches/cliproxyapi" && pwd)
destination=$1
if [ -e "$destination" ] || [ -L "$destination" ]; then
  echo "Destination already exists; choose a new directory." >&2
  exit 1
fi

mkdir -- "$destination"
destination=$(CDPATH= cd -- "$destination" && pwd)
git -C "$destination" init --quiet
git -C "$destination" remote add origin https://github.com/router-for-me/CLIProxyAPI.git
git -C "$destination" fetch --depth=1 origin 9a8ca1b577ef8573c5ccf108450b93ed57716cc4
git -C "$destination" checkout --detach FETCH_HEAD
test "$(git -C "$destination" rev-parse HEAD)" = 9a8ca1b577ef8573c5ccf108450b93ed57716cc4
for patch_file in "$patch_dir/0001-verify-siwc-id-tokens.patch" "$patch_dir/0002-review-scheduler-struct-copies.patch" "$patch_dir/0003-bind-siwc-account-identity.patch"; do
  git -C "$destination" apply --check "$patch_file"
  git -C "$destination" apply --index "$patch_file"
done

cd -- "$destination"
export GOWORK=off
# Fixed versions; Go authenticates downloads through its checksum database.
go get github.com/coreos/go-oidc/v3@v3.16.0 github.com/go-jose/go-jose/v4@v4.1.3
go mod tidy
go mod verify
# Format the applied Go changes, including new files staged by git apply.
git diff --cached --name-only -- '*.go' |
  while IFS= read -r source_file; do
    gofmt -w "$source_file"
  done
git diff --check
echo "Patched connector source prepared in $destination"
echo "No connector was started and no sign-in or inference was performed."
