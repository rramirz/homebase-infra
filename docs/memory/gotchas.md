# Gotchas

## OpenCode Model IDs

### Symptom

Background agents retry or fail with model-not-found errors.

### Cause

Stale or unavailable model IDs in OpenCode routing, especially old `openai/gpt-5.2`, old `anthropic/claude-opus-4-7`, or unavailable active `github-copilot/*` routes.

### Fix

Use verified active IDs: `openai/gpt-5.5`, `anthropic/claude-opus-4-8`, `anthropic/claude-sonnet-4-6`, `anthropic/claude-haiku-4-5`, `google/gemini-3.1-pro-preview`, and `google/gemini-3-flash-preview`.

### Related Files

- `/Users/rafaelramirez/.config/opencode/oh-my-openagent.json`
