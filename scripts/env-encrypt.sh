#!/usr/bin/env bash
set -euo pipefail

for dir in "$@"; do
  if [ ! -f "$dir/.env" ]; then
    echo "skip $dir: no .env"
    continue
  fi
  # sops re-randomizes IV/MAC on every encrypt, so only rewrite .env.enc when the plaintext changed.
  if [ -f "$dir/.env.enc" ] && sops decrypt --input-type dotenv --output-type dotenv "$dir/.env.enc" 2>/dev/null | cmp -s - "$dir/.env"; then
    echo "unchanged $dir"
    continue
  fi
  sops encrypt --filename-override "$dir/.env" --input-type dotenv --output-type dotenv --output "$dir/.env.enc" "$dir/.env"
  echo "encrypted $dir/.env.enc"
done
