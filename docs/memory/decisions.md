# Decisions

## 2026-06-12 - OpenCode Efficient Routing

### Decision

Use `sisyphus` as a cost-conscious orchestrator on `openai/gpt-5.5` medium, with specialized background agents/categories routed to cheaper appropriate models. Keep Fable OFF by default and opt-in only.

### Reason

Default worker sessions create most token volume. Cached input works best with stable, concise context and premium models reserved for explicit hard reasoning.

### Impact

OpenCode should spend less on routine work, retry less on invalid model IDs, and preserve Fable as a deliberate choice.

### Related Files

- `/Users/rafaelramirez/.config/opencode/oh-my-openagent.json`
- `/Users/rafaelramirez/.config/opencode/scripts/fable-swap.py`
- `/Users/rafaelramirez/.config/opencode/AGENTS.md`
