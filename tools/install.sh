#!/usr/bin/env bash
# Installation of helper tools for the programmer (esptool, mpremote in venv)
# and compilation of the Go binary.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(dirname "$DIR")"

echo "==> Creating Python environment in tools/.venv"
python3 -m venv "$DIR/.venv"
"$DIR/.venv/bin/pip" install --quiet --upgrade pip
"$DIR/.venv/bin/pip" install --quiet esptool mpremote
echo "==> esptool: $("$DIR/.venv/bin/esptool" version 2>/dev/null | head -1 || echo installed)"

if command -v go >/dev/null 2>&1; then
    echo "==> Compiling espflasher"
    (cd "$ROOT" && go build -o espflasher .)
    echo "==> Done: $ROOT/espflasher"
else
    echo "!! Go is not in PATH — install it, then: cd $ROOT && go build -o espflasher ."
fi

echo
echo "To run: $ROOT/espflasher"
echo "Note: access to serial ports requires uucp/dialout group:"
echo "  sudo usermod -aG uucp \$USER   # Arch"
echo "  sudo usermod -aG dialout \$USER # Debian/Ubuntu"
