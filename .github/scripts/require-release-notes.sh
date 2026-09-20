#!/usr/bin/env bash
set -euo pipefail

: "${GITHUB_REF_NAME:?}"

notes_file=".github/release-notes/${GITHUB_REF_NAME}.md"
if [[ ! -f "$notes_file" ]]; then
  echo "::error::$notes_file is missing. Copy .github/release-notes/TEMPLATE.md to it and fill in what changed before tagging."
  exit 1
fi

echo "$notes_file"
