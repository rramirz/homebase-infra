# Family Retirement Planner

Self-hosted retirement and estate-planning workbench for independent households. Deterministic calculations only; not financial, legal, tax, or probate advice.

```bash
docker compose up -d --wait postgres
export DATABASE_URL='postgres://retirement:retirement@localhost:5432/retirement?sslmode=disable'
export COOKIE_SECURE=false
go run ./cmd/server
```

Open `http://localhost:8080/register`. Migrations run at startup.

```bash
task test
TEST_DATABASE_URL="$DATABASE_URL" go test -race -tags=integration ./...
```

Authenticated households include an Estate area for beneficiaries, trusts, assets and owners, designations, insurance, document/contact/digital metadata, residuary shares, death scenarios, and persisted printable results. Digital metadata must never contain credentials.

See `ARCHITECTURE.md`. Helm chart exists for static validation only; it has not been deployed.
