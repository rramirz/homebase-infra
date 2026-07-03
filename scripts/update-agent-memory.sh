#!/usr/bin/env bash
set -euo pipefail

mkdir -p docs/memory

touch docs/memory/current-work.md
touch docs/memory/decisions.md
touch docs/memory/gotchas.md
touch docs/memory/changelog.md

today="$(date +%F)"

if ! grep -q "# Current Work" docs/memory/current-work.md; then
  cat > docs/memory/current-work.md <<EOF
# Current Work

## Active Goal

TBD

## Current Status

- TBD

## Next Steps

- TBD

## Open Questions

- TBD

## Last Updated

$today
EOF
fi

if ! grep -q "# Decisions" docs/memory/decisions.md; then
  printf '# Decisions\n' > docs/memory/decisions.md
fi

if ! grep -q "# Gotchas" docs/memory/gotchas.md; then
  printf '# Gotchas\n' > docs/memory/gotchas.md
fi

if ! grep -q "# Agent Changelog" docs/memory/changelog.md; then
  printf '# Agent Changelog\n' > docs/memory/changelog.md
fi

echo "Agent memory files are ready."
echo "Reminder: keep docs/memory/current-work.md short and remove stale context."
