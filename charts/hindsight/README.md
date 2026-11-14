# hindsight

Agent long-term memory (Hindsight 1.2.0). Namespace `hindsight`, Helm release `hindsight`.
Hosts (LAN/VPN only, kgateway `home`): `hindsight.theramirez.casa`, aliases `lb-mem` / `arrive-mem`.

## Deploy

```sh
helm --kube-context home-base -n hindsight upgrade hindsight \
  oci://ghcr.io/vectorize-io/charts/hindsight --version 1.2.0 -f charts/hindsight/values.yaml --wait
kubectl --context home-base apply -f charts/hindsight/httproutes.yaml
```

Prerequisite: secret `hindsight-llm` in `hindsight` (LLM API key; out-of-band, not in git).

## Reranker tuning (2026-11-13)

Recall was ~64s: default local cross-encoder scored 300 candidates on CPU (home-base5, Ryzen 7 5700U, 2-CPU limit).
Alias route also lacked a timeout, so kgateway's 15s default returned 504.

| Config | Rerank | Recall |
|---|---|---|
| local hs-rerank-1, 300 candidates | 62.9s | 64.5s |
| flashrank MiniLM-L-12, 300 | 22-25s | 24-27s |
| flashrank TinyBERT-L-2, 100 (current) | ~0.3s | ~1.8s |

If ranking quality drops, raise `HINDSIGHT_API_RERANKER_MAX_CANDIDATES` (cost is linear).
OpenCode Go cannot serve as reranker (chat-only, no `/rerank`).
