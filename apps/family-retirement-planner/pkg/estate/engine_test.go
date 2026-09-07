package estate

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestRunEightStagesAndLiquidity(t *testing.T) {
	alex := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	blair := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	trustID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	in := Input{
		AsOfYear:      2026,
		DeathYear:     2030,
		DecedentName:  "Morgan",
		Beneficiaries: []Beneficiary{{ID: alex, Name: "Alex"}, {ID: blair, Name: "Blair"}},
		Trusts:        []Trust{{ID: trustID, Name: "Family Trust", Designations: []Designation{{BeneficiaryID: alex, SharePercent: decimal.RequireFromString("0.6"), Tier: TierPrimary}, {BeneficiaryID: blair, SharePercent: decimal.RequireFromString("0.4"), Tier: TierPrimary}, {BeneficiaryID: alex, SharePercent: decimal.NewFromInt(1), Tier: TierContingent}}}},
		Assets: []Asset{
			{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Name: "Joint home", Value: decimal.NewFromInt(500), TransferMethod: TransferWROS, Owners: []Owner{{Name: "Morgan"}, {Name: "Casey"}}},
			{ID: uuid.MustParse("10000000-0000-0000-0000-000000000002"), Name: "TOD account", Value: decimal.NewFromInt(100), Liquid: true, TransferMethod: TransferDesignate, Designations: []Designation{{BeneficiaryID: alex, SharePercent: decimal.NewFromInt(1), Tier: TierPrimary}, {BeneficiaryID: blair, SharePercent: decimal.NewFromInt(1), Tier: TierContingent}}},
			{ID: uuid.MustParse("10000000-0000-0000-0000-000000000003"), Name: "Trust account", Value: decimal.NewFromInt(200), Liquid: true, TransferMethod: TransferTrust, TrustID: trustID},
			{ID: uuid.MustParse("10000000-0000-0000-0000-000000000004"), Name: "Probate cash", Value: decimal.NewFromInt(50), Liquid: true, TransferMethod: TransferProbate},
			{ID: uuid.MustParse("10000000-0000-0000-0000-000000000005"), Name: "Probate property", Value: decimal.NewFromInt(300), TransferMethod: TransferProbate},
		},
		Insurance: []Insurance{
			{ID: uuid.MustParse("20000000-0000-0000-0000-000000000001"), PolicyName: "Direct policy", Benefit: decimal.NewFromInt(80), Designations: []Designation{{BeneficiaryID: blair, SharePercent: decimal.NewFromInt(1), Tier: TierPrimary}, {BeneficiaryID: alex, SharePercent: decimal.NewFromInt(1), Tier: TierContingent}}},
			{ID: uuid.MustParse("20000000-0000-0000-0000-000000000002"), PolicyName: "Estate policy", Benefit: decimal.NewFromInt(70), InEstate: true},
		},
		Residuary:   []ResiduaryShare{{BeneficiaryID: alex, SharePercent: decimal.RequireFromString("0.5")}, {BeneficiaryID: blair, SharePercent: decimal.RequireFromString("0.5")}},
		Debt:        decimal.NewFromInt(150),
		FuneralCost: decimal.NewFromInt(10),
		LegalCost:   decimal.NewFromInt(10),
		PayDebts:    true,
	}

	got, err := Run(in)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Valid || got.Insolvent {
		t.Fatalf("unexpected validity: %#v", got)
	}
	if got.AsOfYear != 2026 || got.DeathYear != 2030 || len(got.ObligationDetails) != 3 {
		t.Fatalf("missing scenario metadata: %#v", got)
	}
	if !got.GrossEstate.Equal(decimal.NewFromInt(1300)) || !got.ProbateEstate.Equal(decimal.NewFromInt(420)) || !got.NetEstate.Equal(decimal.NewFromInt(250)) {
		t.Fatalf("unexpected totals: gross=%s probate=%s net=%s", got.GrossEstate, got.ProbateEstate, got.NetEstate)
	}
	if !got.LiquidityWarning || !got.LiquidityShortfall.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("expected 50 liquidity shortfall: %#v", got)
	}
	stages := []string{}
	for _, event := range got.Events {
		if len(stages) == 0 || stages[len(stages)-1] != event.Stage {
			stages = append(stages, event.Stage)
		}
	}
	wantStages := []string{StageInsurance, StageWROS, StageDesignation, StageTrust, StageProbate, StageObligations, StageResiduary, StageWarnings}
	if !reflect.DeepEqual(stages, wantStages) {
		t.Fatalf("stage order = %#v, want %#v", stages, wantStages)
	}
	if got.Events[2].Target != "Casey" {
		t.Fatalf("WROS target = %q", got.Events[2].Target)
	}
}

func TestRunInsolventDoesNotDistributeNegativeResidue(t *testing.T) {
	b := uuid.New()
	got, err := Run(Input{
		Beneficiaries: []Beneficiary{{ID: b, Name: "Beneficiary"}},
		Assets:        []Asset{{ID: uuid.New(), Name: "Cash", Value: decimal.NewFromInt(25), Liquid: true, TransferMethod: TransferProbate}},
		Residuary:     []ResiduaryShare{{BeneficiaryID: b, SharePercent: decimal.NewFromInt(1)}},
		Debt:          decimal.NewFromInt(100),
		PayDebts:      true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Insolvent || !got.LiquidityWarning || !got.NetEstate.IsZero() {
		t.Fatalf("unexpected result: %#v", got)
	}
	for _, event := range got.Events {
		if event.Stage == StageResiduary && !event.Amount.IsZero() {
			t.Fatalf("negative residue distributed: %#v", event)
		}
	}
}

func TestRunRejectsInvalidReferencesAndShares(t *testing.T) {
	b := uuid.New()
	got, err := Run(Input{
		Beneficiaries: []Beneficiary{{ID: b, Name: "Beneficiary"}},
		Assets:        []Asset{{ID: uuid.New(), Name: "Trust asset", Value: decimal.NewFromInt(1), TransferMethod: TransferTrust, TrustID: uuid.New()}},
		Insurance:     []Insurance{{ID: uuid.New(), PolicyName: "Policy", Benefit: decimal.NewFromInt(1), Designations: []Designation{{BeneficiaryID: uuid.New(), SharePercent: decimal.NewFromInt(1), Tier: TierPrimary}}}},
		Residuary:     []ResiduaryShare{{BeneficiaryID: b, SharePercent: decimal.RequireFromString("0.5")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Valid || len(got.Errors) != 3 || len(got.Events) != 0 {
		t.Fatalf("invalid input accepted: %#v", got)
	}
}

func TestRunContingentTierIsValidatedButNotDistributed(t *testing.T) {
	primary := uuid.New()
	contingent := uuid.New()
	input := Input{
		Beneficiaries: []Beneficiary{{ID: primary, Name: "Primary"}, {ID: contingent, Name: "Contingent"}},
		Assets: []Asset{{ID: uuid.New(), Name: "TOD", Value: decimal.NewFromInt(100), TransferMethod: TransferDesignate, Designations: []Designation{
			{BeneficiaryID: primary, SharePercent: decimal.NewFromInt(1), Tier: TierPrimary},
			{BeneficiaryID: contingent, SharePercent: decimal.NewFromInt(1), Tier: TierContingent},
		}}},
		Residuary: []ResiduaryShare{{BeneficiaryID: primary, SharePercent: decimal.NewFromInt(1)}},
	}
	got, err := Run(input)
	if err != nil || !got.Valid {
		t.Fatalf("run failed: %v %#v", err, got)
	}
	for _, event := range got.Events {
		if event.Target == "Contingent" {
			t.Fatalf("contingent designation distributed: %#v", event)
		}
	}
	if len(got.DesignationTiers) != 2 || !got.DesignationTiers[1].Complete {
		t.Fatalf("contingent readiness missing: %#v", got.DesignationTiers)
	}

	input.Assets[0].Designations[1].SharePercent = decimal.RequireFromString("0.5")
	invalid, _ := Run(input)
	if invalid.Valid {
		t.Fatalf("invalid contingent tier accepted: %#v", invalid)
	}
}

func TestRunCanExcludeDebtsFromScenarioObligations(t *testing.T) {
	b := uuid.New()
	got, err := Run(Input{
		AsOfYear: 2026, DeathYear: 2027,
		Beneficiaries: []Beneficiary{{ID: b, Name: "B"}},
		Assets:        []Asset{{ID: uuid.New(), Name: "Cash", Value: decimal.NewFromInt(100), Liquid: true, TransferMethod: TransferProbate}},
		Residuary:     []ResiduaryShare{{BeneficiaryID: b, SharePercent: decimal.NewFromInt(1)}},
		Debt:          decimal.NewFromInt(80), FuneralCost: decimal.NewFromInt(10), LegalCost: decimal.NewFromInt(5), PayDebts: false,
	})
	if err != nil || !got.Valid {
		t.Fatalf("run failed: %v %#v", err, got)
	}
	if !got.Obligations.Equal(decimal.NewFromInt(15)) || !got.NetEstate.Equal(decimal.NewFromInt(85)) || got.ObligationDetails[2].Included {
		t.Fatalf("debt exclusion failed: %#v", got)
	}
}

func TestRunIsDeterministicByID(t *testing.T) {
	b1 := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	b2 := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	base := Input{
		Beneficiaries: []Beneficiary{{ID: b2, Name: "B"}, {ID: b1, Name: "A"}},
		Assets: []Asset{
			{ID: uuid.MustParse("10000000-0000-0000-0000-000000000002"), Name: "B", Value: decimal.NewFromInt(2), Liquid: true, TransferMethod: TransferProbate},
			{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Name: "A", Value: decimal.NewFromInt(1), Liquid: true, TransferMethod: TransferProbate},
		},
		Residuary: []ResiduaryShare{{BeneficiaryID: b2, SharePercent: decimal.RequireFromString("0.5")}, {BeneficiaryID: b1, SharePercent: decimal.RequireFromString("0.5")}},
	}
	got, _ := Run(base)
	if got.Events[0].Source != "A" || got.Events[2].Target != "A" {
		t.Fatalf("events not sorted deterministically: %#v", got.Events)
	}
}

func TestRunStageOrderIsStableRegardlessOfUUIDOrdering(t *testing.T) {
	alex := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	blair := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	trustID := uuid.MustParse("00000000-0000-0000-0000-000000000003")

	// UUIDs deliberately interleave transfer methods so a single UUID-sorted
	// asset pass would emit stages out of order (e.g. probate before wros).
	assets := []Asset{
		{ID: uuid.MustParse("10000000-0000-0000-0000-000000000005"), Name: "Joint account", Value: decimal.NewFromInt(100), TransferMethod: TransferWROS, Owners: []Owner{{Name: "Morgan"}, {Name: "Casey"}}},
		{ID: uuid.MustParse("10000000-0000-0000-0000-000000000002"), Name: "TOD account", Value: decimal.NewFromInt(100), Liquid: true, TransferMethod: TransferDesignate, Designations: []Designation{{BeneficiaryID: alex, SharePercent: decimal.NewFromInt(1), Tier: TierPrimary}}},
		{ID: uuid.MustParse("10000000-0000-0000-0000-000000000003"), Name: "Trust account", Value: decimal.NewFromInt(100), Liquid: true, TransferMethod: TransferTrust, TrustID: trustID},
		{ID: uuid.MustParse("10000000-0000-0000-0000-000000000001"), Name: "Probate cash", Value: decimal.NewFromInt(100), Liquid: true, TransferMethod: TransferProbate},
		{ID: uuid.MustParse("10000000-0000-0000-0000-000000000004"), Name: "Probate property", Value: decimal.NewFromInt(400), TransferMethod: TransferProbate},
	}
	base := Input{
		AsOfYear: 2026, DeathYear: 2030, DecedentName: "Morgan",
		Beneficiaries: []Beneficiary{{ID: alex, Name: "Alex"}, {ID: blair, Name: "Blair"}},
		Trusts:        []Trust{{ID: trustID, Name: "Family Trust", Designations: []Designation{{BeneficiaryID: alex, SharePercent: decimal.NewFromInt(1), Tier: TierPrimary}}}},
		Assets:        assets,
		Insurance: []Insurance{
			{ID: uuid.MustParse("20000000-0000-0000-0000-000000000001"), PolicyName: "Direct policy", Benefit: decimal.NewFromInt(50), Designations: []Designation{{BeneficiaryID: blair, SharePercent: decimal.NewFromInt(1), Tier: TierPrimary}}},
		},
		Residuary:   []ResiduaryShare{{BeneficiaryID: alex, SharePercent: decimal.NewFromInt(1)}},
		FuneralCost: decimal.NewFromInt(100), LegalCost: decimal.NewFromInt(100),
	}
	reversed := base
	reversed.Assets = append([]Asset(nil), assets...)
	for i, j := 0, len(reversed.Assets)-1; i < j; i, j = i+1, j-1 {
		reversed.Assets[i], reversed.Assets[j] = reversed.Assets[j], reversed.Assets[i]
	}

	wantStages := []string{StageInsurance, StageWROS, StageDesignation, StageTrust, StageProbate, StageObligations, StageResiduary, StageWarnings}
	for name, input := range map[string]Input{"forward": base, "reversed": reversed} {
		got, err := Run(input)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !got.Valid {
			t.Fatalf("%s: unexpected invalid result: %#v", name, got)
		}
		stages := []string{}
		for _, event := range got.Events {
			if len(stages) == 0 || stages[len(stages)-1] != event.Stage {
				stages = append(stages, event.Stage)
			}
		}
		if !reflect.DeepEqual(stages, wantStages) {
			t.Fatalf("%s: stage order = %#v, want %#v", name, stages, wantStages)
		}
	}
}

func TestRunRejectsWROSAssetWithoutDecedentAmongOwners(t *testing.T) {
	b := uuid.New()
	got, err := Run(Input{
		Beneficiaries: []Beneficiary{{ID: b, Name: "Beneficiary"}},
		DecedentName:  "Morgan",
		Assets: []Asset{{ID: uuid.New(), Name: "Joint account", Value: decimal.NewFromInt(100), TransferMethod: TransferWROS,
			Owners: []Owner{{Name: "Morgann"}, {Name: "Casey"}}}}, // typo: decedent not recorded
		Residuary: []ResiduaryShare{{BeneficiaryID: b, SharePercent: decimal.NewFromInt(1)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Valid {
		t.Fatalf("WROS asset without the decedent among owners accepted: %#v", got)
	}
	if len(got.Events) != 0 {
		t.Fatalf("events emitted for invalid input: %#v", got.Events)
	}
}

func TestRunRejectsWROSAssetWithDecedentAsSoleOwner(t *testing.T) {
	got, err := Run(Input{
		DecedentName: "Morgan",
		Assets: []Asset{{ID: uuid.New(), Name: "Sole account", Value: decimal.NewFromInt(100), TransferMethod: TransferWROS,
			Owners: []Owner{{Name: "Morgan"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Valid {
		t.Fatalf("WROS asset with no distinct survivor accepted: %#v", got)
	}
	if len(got.Events) != 0 {
		t.Fatalf("events emitted for invalid input: %#v", got.Events)
	}
}

func TestRunWROSTransferWithDecedentAndSurvivor(t *testing.T) {
	b := uuid.New()
	got, err := Run(Input{
		Beneficiaries: []Beneficiary{{ID: b, Name: "Beneficiary"}},
		DecedentName:  "morgan", // case-insensitive match against recorded owner
		Assets: []Asset{{ID: uuid.New(), Name: "Joint account", Value: decimal.NewFromInt(100), TransferMethod: TransferWROS,
			Owners: []Owner{{Name: "Morgan"}, {Name: "Casey"}}}},
		Residuary: []ResiduaryShare{{BeneficiaryID: b, SharePercent: decimal.NewFromInt(1)}},
	})
	if err != nil || !got.Valid {
		t.Fatalf("valid WROS transfer failed: %v %#v", err, got)
	}
	var wros *Event
	for i := range got.Events {
		if got.Events[i].Stage == StageWROS {
			wros = &got.Events[i]
		}
	}
	if wros == nil {
		t.Fatalf("no WROS event emitted: %#v", got.Events)
	}
	if wros.Target != "Casey" {
		t.Fatalf("WROS target = %q, want %q", wros.Target, "Casey")
	}
	if !wros.Amount.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("WROS amount = %s, want 100", wros.Amount)
	}
}
