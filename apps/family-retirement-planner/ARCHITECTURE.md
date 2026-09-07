# Family Retirement Planner Architecture

## Purpose

Single Go monolith for entering household retirement and estate-planning inventory, generating transparent deterministic projections, and comparing or inspecting scenarios. It does not provide financial, legal, tax, or probate advice.

## Components

- Gin HTTP server: registration/login, secure opaque sessions, CSRF, server-rendered forms.
- PostgreSQL: users, households, retirement records, estate beneficiaries/trusts/assets/owners/designations/insurance/documents/contacts/digital metadata/residuary shares/scenarios/results, and schema migration versions.
- `pkg/retirement`: pure decimal projection engine with no HTTP/database dependencies.
- `pkg/estate`: pure decimal death-event transfer estimator with no HTTP/database dependencies.
- Embedded templates/static CSS: retirement workflows plus a dedicated household estate area and printable persisted estate results.
- Adjacent Helm chart: app Deployment/Service/HTTPRoute plus single PostgreSQL StatefulSet/PVC.

## Isolation and Authentication

User credentials use Argon2id. Session cookies are opaque 256-bit values; only SHA-256 hashes are stored. Production cookies use `Secure`, `HttpOnly`, `SameSite=Lax`, and `__Host-` names. Every state-changing request requires a CSRF token.

Every household-owned table carries `user_id` and `household_id`. Repository operations accept both IDs and predicate every list/update/delete query on both. Composite foreign keys prevent cross-household references. A cross-household request returns not-found.

Estate routes exist only below authenticated `/households/:id/estate`. Every estate handler calls `scopeFor` before a typed repository operation. There is no public estate route. Digital records contain metadata and the location of access instructions only; forms explicitly prohibit passwords, PINs, recovery codes, private keys, security answers, and other credentials, and the repository checks every digital metadata field and rejects credential-like values before insert or update.

## Projection Flow

Inputs are loaded and normalized into decimal values, then scenario overrides are applied to a copy. For each calendar year:

1. Record opening liquid balances and member ages.
2. Activate and inflate income and expenses.
3. Accrue liability interest and apply capped payments.
4. Calculate net cash flow.
5. Sweep surplus to cash or withdraw cash → taxable → tax-deferred → Roth.
6. Apply account contributions and end-of-year nominal returns.
7. Record balances, shortfall, and explainable timeline events.

Primary residence/property equity never enters the liquid portfolio. Taxes, RMDs, Monte Carlo, Roth conversion optimization, and healthcare submodels are deliberate non-goals for this milestone.

## Scenarios

Scenarios store sparse JSON overrides. Baseline household records remain unchanged. Supported override data includes plan return/inflation, person retirement year, income start year, expense amount, and a one-time expense. Comparison recomputes baseline and each scenario from source data to avoid stale result caches.

## Estate Planning Flow

Estate inventory is adjacent to retirement data rather than overloading generic retirement resources. Dedicated typed repository models cover beneficiaries (including dependent status), trusts, trust allocations, assets, owners, direct designations, life insurance, legal document metadata, contacts, digital metadata, residuary shares, death scenarios, and immutable run results. Trust, asset, and insurance designations have `primary` and `contingent` tiers. Every recorded tier must independently total exactly 100%; primary drives the deterministic transfer, while contingent remains fallback/readiness metadata until survival modeling exists.

`pkg/estate` executes stable stages in this order:

1. Life insurance: direct beneficiary payouts or policy proceeds payable to the estate.
2. Joint ownership with survivorship: transfer to recorded surviving owners.
3. Direct designations: recorded TOD/POD or beneficiary allocations.
4. Trusts: transfer through a recorded trust and allocate using its recorded shares.
5. Probate inventory: remaining liquid and non-liquid assets enter the modeled probate estate.
6. Obligations: subtract user-entered funeral costs, legal/administration costs, and debts only when the scenario's `pay_debts` switch is enabled.
7. Residuary: allocate the non-negative probate residue using shares totaling exactly 100%.
8. Warnings: expose probate liquidity shortfall and modeled insolvency without inventing sale timing, priority, tax, or jurisdiction rules.

Each scenario records an as-of year, death year, funeral cost, legal/administration cost, debt amount, and debt-inclusion switch. Each run snapshots normalized input and structured output in PostgreSQL, including timing metadata, obligation detail, designation-tier status, and readiness at run time. Each run also snapshots the scenario's name, decedent name, and as-of/death years into the result row itself, so later scenario edits or deletion cannot mutate a persisted report; its composite `(scenario_id, user_id, household_id)` foreign key remains enforced while scenario deletion sets only `scenario_id` to `NULL`. Result and print routes require the same user/household scope as the scenario. The report is an informational estimate, not a will, trust, title opinion, beneficiary form, legal conclusion, or tax/probate calculation.

## Estate Readiness

`pkg/estate/readiness.go` is a pure typed evaluator. It returns ordered checklist items with `pass`, `attention`, or `not_applicable` status for signed will inventory, executor contact, guardian contact when a dependent beneficiary exists, financial power of attorney inventory, healthcare directive inventory, primary/contingent asset and insurance shares, residuary shares, and scenario probate liquidity. Document checks are household-level because documents are not linked to people; they explicitly do not claim per-adult coverage or legal validity. The dashboard evaluates the earliest recorded scenario, and each persisted run stores readiness for its selected scenario.

## Deployment

Local development uses Docker Compose PostgreSQL. Kubernetes topology is one app replica plus one PostgreSQL StatefulSet using `ssd` storage. HTTPRoute attaches to kgateway `home` at `family-retirement-planner.theramirez.casa`.

The route is LAN/VPN-only. Never add it to Cloudflare Tunnel. The chart is not deployed. Do not enter real financial data until encrypted off-cluster backups and a tested restore exist.

## Failure Modes

- PostgreSQL unavailable: readiness fails; app data operations fail closed.
- Migration failure: process startup stops.
- Invalid scenario/input: projection request returns bad request; baseline remains unchanged.
- Invalid estate shares/references: no estate result is persisted; the correction page lists deterministic validation failures.
- Estate liquidity warning: result persists with explicit shortfall; the engine does not assume a forced asset sale.
- Entered obligations exceed probate assets: residue is clamped to zero and a solvency warning is persisted.
- Local `ssd` PVC deletion: data loss. No production-grade backup exists yet.
- Cookie secure mode over plain HTTP: login appears unsuccessful; use TLS or set `COOKIE_SECURE=false` only locally.
