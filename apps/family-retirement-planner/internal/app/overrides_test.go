package app_test

import (
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/internal/app"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/pkg/retirement"
	"github.com/shopspring/decimal"
	"testing"
)

func TestApplyOverrides(t *testing.T) {
	input := &retirement.Input{
		Assumptions: retirement.Assumptions{AnnualReturn: decimal.NewFromFloat(0.05)},
		People:      []retirement.Person{{ID: "00000000-0000-0000-0000-000000000001", Name: "Dad", RetirementYear: 2040}},
		Incomes:     []retirement.Income{{Name: "employment", Kind: "employment", OwnerID: "00000000-0000-0000-0000-000000000001", EndYear: 2040}},
	}
	jsonStr := `{"annual_return":"0.07","retirement_year":2045,"person_id":"00000000-0000-0000-0000-000000000001"}`
	app.ApplyOverrides(input, []byte(jsonStr), nil, nil)

	if !input.Assumptions.AnnualReturn.Equal(decimal.RequireFromString("0.07")) {
		t.Errorf("expected 0.07 annual return")
	}
	if input.Incomes[0].EndYear != 2045 {
		t.Errorf("expected 2045 end year, got %d", input.Incomes[0].EndYear)
	}
	if input.People[0].RetirementYear != 2045 {
		t.Errorf("expected person retirement year 2045, got %d", input.People[0].RetirementYear)
	}
}
