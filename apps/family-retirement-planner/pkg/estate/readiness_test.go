package estate

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestEvaluateReadinessCompleteHouseholdInventory(t *testing.T) {
	b := Beneficiary{ID: uuid.New(), Name: "Child", Dependent: true}
	plan := Input{
		Beneficiaries: []Beneficiary{b},
		Insurance:     []Insurance{{PolicyName: "Policy", Benefit: decimal.NewFromInt(100), Designations: completeTiers(b.ID)}},
		Residuary:     []ResiduaryShare{{BeneficiaryID: b.ID, SharePercent: decimal.NewFromInt(1)}},
		FuneralCost:   decimal.NewFromInt(10), LegalCost: decimal.NewFromInt(5), Debt: decimal.NewFromInt(20), PayDebts: true,
		Assets: []Asset{
			{Name: "TOD", Value: decimal.NewFromInt(100), TransferMethod: TransferDesignate, Designations: completeTiers(b.ID)},
			{Name: "Probate cash", Value: decimal.NewFromInt(50), Liquid: true, TransferMethod: TransferProbate},
		},
	}
	items := EvaluateReadiness(ReadinessInput{
		Plan: plan, HasScenario: true, ScenarioName: "Death year",
		Documents: []ReadinessDocument{{DocumentType: "Last Will", Status: "signed"}, {DocumentType: "Financial Power of Attorney", Status: "signed"}, {DocumentType: "Advance Directive", Status: "signed"}},
		Contacts:  []ReadinessContact{{Role: "executor"}, {Role: "guardian"}},
	})
	for _, item := range items {
		if item.Status != ChecklistPass {
			t.Fatalf("%s = %s: %s", item.Key, item.Status, item.Reason)
		}
	}
}

func TestEvaluateReadinessReportsMissingAndNotApplicable(t *testing.T) {
	b := Beneficiary{ID: uuid.New(), Name: "Adult", Dependent: false}
	items := EvaluateReadiness(ReadinessInput{Plan: Input{
		Beneficiaries: []Beneficiary{b},
		Assets:        []Asset{{Name: "TOD", TransferMethod: TransferDesignate, Designations: []Designation{{BeneficiaryID: b.ID, Tier: TierPrimary, SharePercent: decimal.RequireFromString("0.5")}}}},
		Insurance:     []Insurance{{PolicyName: "Estate policy", InEstate: true}},
	}})
	statuses := map[string]string{}
	for _, item := range items {
		statuses[item.Key] = item.Status
	}
	if statuses["guardian"] != ChecklistNotApplicable || statuses["asset_primary"] != ChecklistAttention || statuses["asset_contingent"] != ChecklistAttention || statuses["insurance_primary"] != ChecklistNotApplicable || statuses["liquidity"] != ChecklistAttention {
		t.Fatalf("unexpected statuses: %#v", statuses)
	}
}

func TestEvaluateReadinessHouseholdDocumentLimitation(t *testing.T) {
	items := EvaluateReadiness(ReadinessInput{Documents: []ReadinessDocument{{DocumentType: "Living Will", Status: "signed"}}})
	byKey := map[string]ChecklistItem{}
	for _, item := range items {
		byKey[item.Key] = item
	}
	if byKey["signed_will"].Status != ChecklistAttention || byKey["healthcare_directive"].Status != ChecklistPass {
		t.Fatalf("document classification failed: %#v", byKey)
	}
	if !strings.Contains(byKey["healthcare_directive"].Reason, "household-level") {
		t.Fatalf("missing household limitation: %s", byKey["healthcare_directive"].Reason)
	}
}

func completeTiers(id uuid.UUID) []Designation {
	return []Designation{{BeneficiaryID: id, Tier: TierPrimary, SharePercent: decimal.NewFromInt(1)}, {BeneficiaryID: id, Tier: TierContingent, SharePercent: decimal.NewFromInt(1)}}
}
