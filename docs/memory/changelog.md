# Agent Changelog

## 2026-06-12

### Changed

- Added lightweight repo memory/topic-doc scaffold.
- Documented OpenCode routing and Fable opt-in decision.

### Files Modified

- `docs/architecture.md`
- `docs/database.md`
- `docs/deployment.md`
- `docs/memory/current-work.md`
- `docs/memory/decisions.md`
- `docs/memory/gotchas.md`
- `docs/memory/changelog.md`
- `scripts/update-agent-memory.sh`

### Commands Run

- `chmod +x scripts/update-agent-memory.sh`
- `scripts/update-agent-memory.sh`
- `python3 -m py_compile ~/.config/opencode/scripts/fable-swap.py`
- JSON parse checks for OpenCode config/profile files.
- `python3 ~/.config/opencode/scripts/fable-swap.py status`

### Notes

- Keep this file short; do not duplicate Git history.
