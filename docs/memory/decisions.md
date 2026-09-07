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

## 2026-08-13 - Estate Planning Is a Separate Deterministic Module

### Decision

Add estate planning beside retirement planning using dedicated scoped tables, typed repositories, and pure `pkg/estate` logic. Share household identity and authentication, but do not feed estate assets into retirement liquidity or put estate-law behavior into `pkg/retirement`.

### Reason

Estate inventory and death-event cash-flow questions reuse household context but have different semantics and safety boundaries. Separation keeps retirement calculations stable and prevents inventory estimates from being presented as legal conclusions.

### Impact

The module supports primary/contingent designations, dependent-aware readiness, explicit death timing and obligations, eight-stage transfer estimates, and persisted printable results. It does not calculate probate law, intestacy, legal validity, estate/inheritance taxes, or store credentials/document contents.
