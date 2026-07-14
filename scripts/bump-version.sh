#!/usr/bin/env sh
set -eu

version_file="${1:-config/version}"

if [ ! -f "$version_file" ]; then
  echo "version file not found: $version_file" >&2
  exit 1
fi

version="$(tr -d '[:space:]' < "$version_file")"
case "$version" in
  2.8.11.*.*) ;;
  2.8.11) version="2.8.11.1.0" ;;
  *)
    echo "unsupported version format: $version" >&2
    echo "expected 2.8.11.x.n" >&2
    exit 1
    ;;
esac

IFS='.' read -r major minor patch x n <<EOF_VERSION
$version
EOF_VERSION

n=$((n + 1))
next="${major}.${minor}.${patch}.${x}.${n}"
printf '%s\n' "$next" > "$version_file"
echo "$next"
