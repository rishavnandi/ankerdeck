#!/bin/sh
set -eu

if [ "$#" -ne 1 ]; then
  printf 'Usage: %s path/to/snapshot.json\n' "$0" >&2
  exit 2
fi

if [ -z "${DATABASE_URL:-}" ]; then
  printf 'DATABASE_URL must be set to the production Neon connection URL.\n' >&2
  exit 2
fi

snapshot_file=$1
if [ ! -f "$snapshot_file" ]; then
  printf 'Snapshot file not found: %s\n' "$snapshot_file" >&2
  exit 2
fi
snapshot_file=$(CDPATH= cd "$(dirname "$snapshot_file")" && pwd)/$(basename "$snapshot_file")

script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
repo_root=$(CDPATH= cd "$script_dir/.." && pwd)
cd "$repo_root"

exec go run . -seed "$snapshot_file"
