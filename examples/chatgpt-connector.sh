#!/bin/sh
# Run with sh on the pilot workstation. exec preserves the background PID.
set -eu
case "${1:-}" in
  login) set -- -codex-login -no-browser ;;
  serve) set -- -local-model ;;
  *) echo 'usage: sh examples/chatgpt-connector.sh login|serve' >&2; exit 2 ;;
esac
case "${CONNECTOR_BIN:?set CONNECTOR_BIN to the pinned connector executable}" in
  /*) ;;
  *) echo 'CONNECTOR_BIN must be absolute' >&2; exit 2 ;;
esac
routeinlet="$(pwd)/routeinlet"
cd "${PILOT_DIR:?set PILOT_DIR to the generated profile directory}"
"$routeinlet" check-chatgpt-pilot
exec env -i HOME="$HOME" PATH=/usr/bin:/bin "$CONNECTOR_BIN" -config "$PWD/config.yaml" "$@"
