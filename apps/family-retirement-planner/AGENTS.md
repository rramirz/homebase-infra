# Family Retirement Planner Agent Context

Read `ARCHITECTURE.md` before changes.

Hard invariants:

- Financial money uses PostgreSQL `NUMERIC` and `decimal.Decimal`, never float persistence.
- `pkg/retirement` stays independent from Gin/PostgreSQL/templates/auth.
- `pkg/estate` stays independent from Gin/PostgreSQL/templates/auth; stage order is insurance → WROS → designation → trust → probate → obligations → residuary → warnings.
- Estate designation tiers are `primary` and `contingent`; validate every recorded tier at exactly 100%, distribute primary only, and keep contingent as readiness/fallback metadata until survival modeling exists.
- Estate readiness is pure in `pkg/estate/readiness.go`; document checks are household-level and must never imply per-adult legal coverage or validity.
- Every household query scopes both authenticated user ID and household ID.
- Home/property equity stays outside liquid retirement assets.
- Engine event ordering is stable and tested; document intentional changes.
- No tax/RMD/Monte Carlo/LLM calculations in MVP.
- Estate module never calculates tax, probate law, intestacy, document validity, or jurisdiction rules.
- WROS requires the decedent to be a recorded owner and at least one distinct recorded survivor.
- Estate results snapshot scenario display metadata and remain user+household scoped after scenario deletion.
- Digital metadata rejects credential-like values across every field before create or update.
- No live deployment. LAN/VPN-only; never add hostname to Cloudflare Tunnel.
- No real financial data until encrypted off-cluster backup and restore test exist.

Checks:

```bash
go vet ./...
go test ./...
go test -race ./...
go test -race -tags=integration ./... # requires PostgreSQL
GOTOOLCHAIN=go1.26.6 govulncheck ./...
helm lint chart -f chart/values.yaml -f chart/secret-values.example.yaml
```
