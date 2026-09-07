//go:build integration

package app_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/internal/app"
)

func TestRepositoryIsolation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL or DATABASE_URL required")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := app.NewRepo(db)
	if err := repo.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	u1, err := repo.Register(ctx, uuid.NewString()+"@example.test", "password1234")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := repo.Register(ctx, uuid.NewString()+"@example.test", "password1234")
	if err != nil {
		t.Fatal(err)
	}
	h1, err := repo.CreateHousehold(ctx, u1, "owner")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := repo.CreateHousehold(ctx, u2, "other")
	if err != nil {
		t.Fatal(err)
	}
	owner := app.Scope{UserID: u1, HouseholdID: h1}
	wrongUser := app.Scope{UserID: u2, HouseholdID: h1}
	wrongHousehold := app.Scope{UserID: u1, HouseholdID: h2}

	fixtures := map[string]app.Resource{
		"people":      {Name: "Person", Amount: "1970", StartYear: "2035", EndYear: "2070", Confidence: "verified"},
		"accounts":    {Name: "Cash", Kind: "cash", Amount: "1000", Confidence: "verified"},
		"liabilities": {Name: "Mortgage", Amount: "500", Inflation: "0.03", EndYear: "100", Confidence: "verified"},
		"incomes":     {Name: "Pension", Kind: "pension", Amount: "12000", StartYear: "2035", EndYear: "2070", Inflation: "0.02", Confidence: "verified"},
		"expenses":    {Name: "Living", Kind: "essential", Amount: "10000", StartYear: "2026", EndYear: "2070", Inflation: "0.02", Confidence: "verified"},
	}
	for table, fixture := range fixtures {
		t.Run(table, func(t *testing.T) {
			if err := repo.CreateResource(ctx, owner, table, fixture); err != nil {
				t.Fatal(err)
			}
			owned, err := repo.ListResources(ctx, owner, table)
			if err != nil || len(owned) != 1 {
				t.Fatalf("owner list: %v %#v", err, owned)
			}
			resource := owned[0]
			resource.Name = "Updated"
			if err := repo.UpdateResource(ctx, owner, table, resource); err != nil {
				t.Fatalf("owner update: %v", err)
			}
			for _, scope := range []app.Scope{wrongUser, wrongHousehold} {
				if rows, err := repo.ListResources(ctx, scope, table); err != nil || len(rows) != 0 {
					t.Fatalf("cross-scope list leaked: %v %#v", err, rows)
				}
				if err := repo.UpdateResource(ctx, scope, table, resource); err != app.ErrNotFound {
					t.Fatalf("cross-scope update: %v", err)
				}
				if err := repo.DeleteResource(ctx, scope, table, resource.ID); err != app.ErrNotFound {
					t.Fatalf("cross-scope delete: %v", err)
				}
			}
			stillOwned, err := repo.ListResources(ctx, owner, table)
			if err != nil || len(stillOwned) != 1 || stillOwned[0].Name != "Updated" {
				t.Fatalf("owner data changed: %v %#v", err, stillOwned)
			}
		})
	}

	planID, err := repo.CreatePlan(ctx, owner, app.Plan{Name: "Base", AsOfYear: 2026, EndYear: 2070, AnnualReturn: "0.05", Inflation: "0.02", Confidence: "verified"})
	if err != nil {
		t.Fatal(err)
	}
	plan := app.Plan{ID: planID, Name: "Attack", AsOfYear: 2026, EndYear: 2070, AnnualReturn: "0.05", Inflation: "0.02"}
	if err := repo.UpdatePlan(ctx, wrongUser, plan); err != app.ErrNotFound {
		t.Fatalf("cross-scope plan update: %v", err)
	}
	scenarioID, err := repo.CreateScenario(ctx, owner, planID, "Base scenario", []byte(`{"annual_return":"0.04"}`))
	if err != nil {
		t.Fatal(err)
	}
	if rows, err := repo.Scenarios(ctx, wrongHousehold, planID); err != nil || len(rows) != 0 {
		t.Fatalf("cross-scope scenarios leaked: %v %#v", err, rows)
	}
	attack := app.Scenario{ID: scenarioID, Name: "Attack", Overrides: []byte(`{}`)}
	if err := repo.UpdateScenario(ctx, wrongUser, attack); err != app.ErrNotFound {
		t.Fatalf("cross-scope scenario update: %v", err)
	}
	if _, err := repo.CloneScenario(ctx, wrongHousehold, scenarioID, "Attack clone"); err != app.ErrNotFound {
		t.Fatalf("cross-scope scenario clone: %v", err)
	}
}
