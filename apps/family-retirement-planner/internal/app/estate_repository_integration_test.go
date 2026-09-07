//go:build integration

package app_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/internal/app"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/pkg/estate"
	"github.com/shopspring/decimal"
)

func TestEstateRepositoryPersistsScenarioAndIsolatesHouseholds(t *testing.T) {
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

	u1, err := repo.Register(ctx, uuid.NewString()+"@estate.test", "password1234")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := repo.Register(ctx, uuid.NewString()+"@estate.test", "password1234")
	if err != nil {
		t.Fatal(err)
	}
	h1, err := repo.CreateHousehold(ctx, u1, "Estate owner")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := repo.CreateHousehold(ctx, u2, "Separate household")
	if err != nil {
		t.Fatal(err)
	}
	owner := app.Scope{UserID: u1, HouseholdID: h1}
	wrongUser := app.Scope{UserID: u2, HouseholdID: h1}
	wrongHousehold := app.Scope{UserID: u1, HouseholdID: h2}

	b1, err := repo.CreateEstateBeneficiary(ctx, owner, app.EstateBeneficiary{Name: "Alex", Relationship: "child", Dependent: true})
	if err != nil {
		t.Fatal(err)
	}
	b2, err := repo.CreateEstateBeneficiary(ctx, owner, app.EstateBeneficiary{Name: "Blair", Relationship: "child"})
	if err != nil {
		t.Fatal(err)
	}
	otherBeneficiary, err := repo.CreateEstateBeneficiary(ctx, app.Scope{UserID: u2, HouseholdID: h2}, app.EstateBeneficiary{Name: "Other", Relationship: "friend"})
	if err != nil {
		t.Fatal(err)
	}
	trustID, err := repo.CreateEstateTrust(ctx, owner, app.EstateTrust{Name: "Family Trust", Notes: "inventory only"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddEstateTrustDesignation(ctx, owner, trustID, b1, decimal.NewFromInt(1), estate.TierPrimary); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddEstateTrustDesignation(ctx, owner, trustID, b2, decimal.NewFromInt(1), estate.TierContingent); err != nil {
		t.Fatal(err)
	}
	assetID, err := repo.CreateEstateAsset(ctx, owner, app.EstateAsset{Name: "Trust account", Value: decimal.NewFromInt(200000), Liquid: true, TransferMethod: estate.TransferTrust, TrustID: trustID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddEstateAssetOwner(ctx, owner, assetID, "Morgan"); err != nil {
		t.Fatal(err)
	}
	probateID, err := repo.CreateEstateAsset(ctx, owner, app.EstateAsset{Name: "Probate cash", Value: decimal.NewFromInt(50000), Liquid: true, TransferMethod: estate.TransferProbate})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddEstateAssetOwner(ctx, owner, probateID, "Morgan"); err != nil {
		t.Fatal(err)
	}
	todID, err := repo.CreateEstateAsset(ctx, owner, app.EstateAsset{Name: "TOD account", Value: decimal.NewFromInt(100000), Liquid: true, TransferMethod: estate.TransferDesignate})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddEstateAssetDesignation(ctx, owner, todID, b2, decimal.NewFromInt(1), estate.TierPrimary); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddEstateAssetDesignation(ctx, owner, todID, b1, decimal.NewFromInt(1), estate.TierContingent); err != nil {
		t.Fatal(err)
	}
	policyID, err := repo.CreateEstateInsurance(ctx, owner, app.EstateInsurance{PolicyName: "Term policy", Benefit: decimal.NewFromInt(250000)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddEstateInsuranceDesignation(ctx, owner, policyID, b1, decimal.RequireFromString("0.5"), estate.TierPrimary); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddEstateInsuranceDesignation(ctx, owner, policyID, b2, decimal.RequireFromString("0.5"), estate.TierPrimary); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AddEstateInsuranceDesignation(ctx, owner, policyID, b1, decimal.NewFromInt(1), estate.TierContingent); err != nil {
		t.Fatal(err)
	}
	documentID, err := repo.CreateEstateDocument(ctx, owner, app.EstateDocument{DocumentType: "Will", StorageLocation: "Home safe", Status: "signed", Notes: "Review date recorded externally"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateEstateDocument(ctx, owner, app.EstateDocument{DocumentType: "Financial Power of Attorney", StorageLocation: "Home safe", Status: "signed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateEstateDocument(ctx, owner, app.EstateDocument{DocumentType: "Healthcare Directive", StorageLocation: "Home safe", Status: "signed"}); err != nil {
		t.Fatal(err)
	}
	contactID, err := repo.CreateEstateContact(ctx, owner, app.EstateContact{Role: "executor", Name: "Casey", ContactInfo: "casey@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateEstateContact(ctx, owner, app.EstateContact{Role: "guardian", Name: "Jordan", ContactInfo: "jordan@example.test"}); err != nil {
		t.Fatal(err)
	}
	digitalID, err := repo.CreateEstateDigitalAsset(ctx, owner, app.EstateDigitalAsset{ServiceName: "Password manager", AccountIdentifier: "family vault", AccessInstructionsLocation: "sealed envelope location", Notes: "no credentials stored"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertEstateResiduaryShare(ctx, owner, b1, decimal.RequireFromString("0.6")); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertEstateResiduaryShare(ctx, owner, b2, decimal.RequireFromString("0.4")); err != nil {
		t.Fatal(err)
	}
	scenarioID, err := repo.CreateEstateScenario(ctx, owner, app.EstateScenario{Name: "Morgan dies", DecedentName: "Morgan", AsOfYear: 2026, DeathYear: 2030, Debt: decimal.NewFromInt(10000), FuneralCost: decimal.NewFromInt(2000), LegalCost: decimal.NewFromInt(3000), PayDebts: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateEstateBeneficiary(ctx, owner, app.EstateBeneficiary{ID: b1, Name: "Alex Updated", Relationship: "child", Dependent: true}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateEstateTrust(ctx, owner, app.EstateTrust{ID: trustID, Name: "Updated Family Trust", Notes: "reviewed"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateEstateAsset(ctx, owner, app.EstateAsset{ID: probateID, Name: "Updated probate cash", Value: decimal.NewFromInt(55000), Liquid: true, TransferMethod: estate.TransferProbate}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateEstateInsurance(ctx, owner, app.EstateInsurance{ID: policyID, PolicyName: "Updated term policy", Benefit: decimal.NewFromInt(250000)}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateEstateDocument(ctx, owner, app.EstateDocument{ID: documentID, DocumentType: "Last will", StorageLocation: "Home safe", Status: "signed", Notes: "updated"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateEstateContact(ctx, owner, app.EstateContact{ID: contactID, Role: "executor", Name: "Casey Updated", ContactInfo: "casey@example.test", Notes: "updated"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateEstateDigitalAsset(ctx, owner, app.EstateDigitalAsset{ID: digitalID, ServiceName: "Vault metadata", AccountIdentifier: "family vault", AccessInstructionsLocation: "sealed envelope location", Notes: "no credentials stored"}); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateEstateScenario(ctx, owner, app.EstateScenario{ID: scenarioID, Name: "Updated Morgan scenario", DecedentName: "Morgan", AsOfYear: 2026, DeathYear: 2030, Debt: decimal.NewFromInt(10000), FuneralCost: decimal.NewFromInt(2000), LegalCost: decimal.NewFromInt(3000), PayDebts: true}); err != nil {
		t.Fatal(err)
	}

	input, err := repo.LoadEstateInput(ctx, owner, scenarioID)
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Beneficiaries) != 2 || len(input.Assets) != 3 || len(input.Trusts) != 1 || len(input.Insurance) != 1 || len(input.Residuary) != 2 {
		t.Fatalf("incomplete persisted input: %#v", input)
	}
	if !input.Beneficiaries[0].Dependent || input.AsOfYear != 2026 || input.DeathYear != 2030 || !input.PayDebts || len(input.Assets[2].Designations) != 2 {
		t.Fatalf("tier/scenario/dependent fields not loaded: %#v", input)
	}
	result, err := estate.Run(input)
	if err != nil || !result.Valid {
		t.Fatalf("run failed: %v %#v", err, result)
	}
	result.Readiness, err = repo.EstateReadiness(ctx, owner, scenarioID)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range result.Readiness {
		if item.Status != estate.ChecklistPass {
			t.Fatalf("readiness %s: %s (%s)", item.Key, item.Status, item.Reason)
		}
	}
	resultID, err := repo.SaveEstateResult(ctx, owner, scenarioID, input, result)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := repo.EstateResult(ctx, owner, resultID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ID != resultID || persisted.ScenarioID != scenarioID || persisted.ScenarioName != "Updated Morgan scenario" || persisted.DeathYear != 2030 || !persisted.Result.NetEstate.Equal(decimal.NewFromInt(40000)) || len(persisted.Result.Readiness) == 0 {
		t.Fatalf("unexpected persisted result: %#v", persisted)
	}

	for name, scope := range map[string]app.Scope{"wrong user": wrongUser, "wrong household": wrongHousehold} {
		t.Run(name, func(t *testing.T) {
			data, err := repo.EstateData(ctx, scope)
			if err != nil {
				t.Fatal(err)
			}
			if len(data.Beneficiaries)+len(data.Assets)+len(data.Scenarios)+len(data.Results) != 0 {
				t.Fatalf("cross-scope data leaked: %#v", data)
			}
			if _, err := repo.LoadEstateInput(ctx, scope, scenarioID); !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("scenario leaked: %v", err)
			}
			if _, err := repo.EstateResult(ctx, scope, resultID); !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("result leaked: %v", err)
			}
			if err := repo.UpdateEstateBeneficiary(ctx, scope, app.EstateBeneficiary{ID: b1, Name: "Attacker"}); !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("cross-scope update: %v", err)
			}
			if err := repo.DeleteEstateAsset(ctx, scope, assetID); !errors.Is(err, app.ErrNotFound) {
				t.Fatalf("cross-scope delete: %v", err)
			}
		})
	}
	if _, err := repo.AddEstateAssetDesignation(ctx, owner, todID, otherBeneficiary, decimal.NewFromInt(1), estate.TierPrimary); !errors.Is(err, app.ErrNotFound) {
		t.Fatalf("cross-household beneficiary reference accepted: %v", err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO estate_asset_owners(user_id,household_id,asset_id,owner_name) VALUES($1,$2,$3,'forged')`, u2, h2, assetID); err == nil {
		t.Fatal("database accepted cross-household asset foreign key")
	}
	if _, err := db.Exec(ctx, `INSERT INTO estate_asset_designations(user_id,household_id,asset_id,beneficiary_id,share_percent,tier) VALUES($1,$2,$3,$4,1,'backup')`, u1, h1, todID, b1); err == nil {
		t.Fatal("database accepted invalid designation tier")
	}
}

func TestEstateResultSnapshotSurvivesScenarioEditAndDelete(t *testing.T) {
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
	u1, err := repo.Register(ctx, uuid.NewString()+"@snapshot.test", "password1234")
	if err != nil {
		t.Fatal(err)
	}
	h1, err := repo.CreateHousehold(ctx, u1, "Estate owner")
	if err != nil {
		t.Fatal(err)
	}
	owner := app.Scope{UserID: u1, HouseholdID: h1}

	scenarioID, err := repo.CreateEstateScenario(ctx, owner, app.EstateScenario{Name: "Original name", DecedentName: "Morgan", AsOfYear: 2026, DeathYear: 2030})
	if err != nil {
		t.Fatal(err)
	}
	beneficiaryID := uuid.New()
	input := estate.Input{
		AsOfYear:      2026,
		DeathYear:     2030,
		DecedentName:  "Morgan",
		Beneficiaries: []estate.Beneficiary{{ID: beneficiaryID, Name: "Alex", Relationship: "child"}},
		Residuary:     []estate.ResiduaryShare{{BeneficiaryID: beneficiaryID, SharePercent: decimal.NewFromInt(1)}},
	}
	result, err := estate.Run(input)
	if err != nil || !result.Valid {
		t.Fatalf("run failed: %v %#v", err, result)
	}
	resultID, err := repo.SaveEstateResult(ctx, owner, scenarioID, input, result)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := repo.EstateResult(ctx, owner, resultID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ScenarioName != "Original name" || persisted.DecedentName != "Morgan" || persisted.AsOfYear != 2026 || persisted.DeathYear != 2030 {
		t.Fatalf("snapshot not written at save: %#v", persisted)
	}

	// Renaming the scenario must not mutate the previously persisted result.
	if err := repo.UpdateEstateScenario(ctx, owner, app.EstateScenario{ID: scenarioID, Name: "Renamed", DecedentName: "Morgan Renamed", AsOfYear: 2026, DeathYear: 2031}); err != nil {
		t.Fatal(err)
	}
	persisted, err = repo.EstateResult(ctx, owner, resultID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.ScenarioName != "Original name" || persisted.DecedentName != "Morgan" || persisted.AsOfYear != 2026 || persisted.DeathYear != 2030 {
		t.Fatalf("scenario edit mutated persisted result: %#v", persisted)
	}
	list, err := repo.EstateResults(ctx, owner)
	if err != nil || len(list) != 1 || list[0].ScenarioName != "Original name" || list[0].DecedentName != "Morgan" {
		t.Fatalf("results list mutated by scenario edit: %v %#v", err, list)
	}

	// Deleting the scenario must leave prior results readable.
	if err := repo.DeleteEstateScenario(ctx, owner, scenarioID); err != nil {
		t.Fatal(err)
	}
	persisted, err = repo.EstateResult(ctx, owner, resultID)
	if err != nil {
		t.Fatalf("result lost after scenario delete: %v", err)
	}
	if persisted.ScenarioName != "Original name" || persisted.DecedentName != "Morgan" {
		t.Fatalf("snapshot changed after scenario delete: %#v", persisted)
	}
	list, err = repo.EstateResults(ctx, owner)
	if err != nil || len(list) != 1 || list[0].ScenarioName != "Original name" {
		t.Fatalf("results list lost after scenario delete: %v %#v", err, list)
	}
}

func TestEstateDigitalMetadataCredentialRejection(t *testing.T) {
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
	u1, err := repo.Register(ctx, uuid.NewString()+"@digital.test", "password1234")
	if err != nil {
		t.Fatal(err)
	}
	h1, err := repo.CreateHousehold(ctx, u1, "Estate owner")
	if err != nil {
		t.Fatal(err)
	}
	owner := app.Scope{UserID: u1, HouseholdID: h1}

	if _, err := repo.CreateEstateDigitalAsset(ctx, owner, app.EstateDigitalAsset{ServiceName: "Password manager", AccountIdentifier: "family vault", AccessInstructionsLocation: "sealed envelope location", Notes: "no credentials stored"}); err != nil {
		t.Fatalf("legitimate metadata rejected: %v", err)
	}

	rejected := []app.EstateDigitalAsset{
		{ServiceName: "Bank", AccountIdentifier: "checking", AccessInstructionsLocation: "home safe", Notes: "password: hunter2"},
		{ServiceName: "Bank", AccountIdentifier: "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7", AccessInstructionsLocation: "home safe", Notes: ""},
		{ServiceName: "Bank", AccountIdentifier: "checking", AccessInstructionsLocation: "home safe", Notes: "0123456789abcdef0123456789abcdef"},
		{ServiceName: "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7", AccountIdentifier: "checking", AccessInstructionsLocation: "home safe", Notes: ""},
		{ServiceName: "Bank", AccountIdentifier: "checking", AccessInstructionsLocation: "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7", Notes: ""},
	}
	for i, item := range rejected {
		if _, err := repo.CreateEstateDigitalAsset(ctx, owner, item); !errors.Is(err, app.ErrCredentialDetected) {
			t.Fatalf("create case %d: expected ErrCredentialDetected, got %v for %#v", i, err, item)
		}
	}

	// The update path must reject credentials the same way.
	created, err := repo.CreateEstateDigitalAsset(ctx, owner, app.EstateDigitalAsset{ServiceName: "Vault", AccountIdentifier: "family vault", AccessInstructionsLocation: "home safe", Notes: "ok"})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateEstateDigitalAsset(ctx, owner, app.EstateDigitalAsset{ID: created, ServiceName: "Vault", AccountIdentifier: "family vault", AccessInstructionsLocation: "home safe", Notes: "private key here"}); !errors.Is(err, app.ErrCredentialDetected) {
		t.Fatalf("update accepted credential: %v", err)
	}
	if err := repo.UpdateEstateDigitalAsset(ctx, owner, app.EstateDigitalAsset{ID: created, ServiceName: "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7", AccountIdentifier: "family vault", AccessInstructionsLocation: "home safe", Notes: "ok"}); !errors.Is(err, app.ErrCredentialDetected) {
		t.Fatalf("update accepted credential in ServiceName: %v", err)
	}
	if err := repo.UpdateEstateDigitalAsset(ctx, owner, app.EstateDigitalAsset{ID: created, ServiceName: "Vault", AccountIdentifier: "family vault", AccessInstructionsLocation: "aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7", Notes: "ok"}); !errors.Is(err, app.ErrCredentialDetected) {
		t.Fatalf("update accepted credential in AccessInstructionsLocation: %v", err)
	}

	// Only the two legitimate rows may exist; nothing credential-like persisted.
	assets, err := repo.EstateDigitalAssets(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 {
		t.Fatalf("credential-like values persisted: %#v", assets)
	}
}
