package app

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/pkg/retirement"
	"github.com/shopspring/decimal"
)

type Overrides struct {
	AnnualReturn    string    `json:"annual_return"`
	Inflation       string    `json:"inflation"`
	RetirementYear  int       `json:"retirement_year"`
	PersonID        uuid.UUID `json:"person_id"`
	IncomeID        uuid.UUID `json:"income_id"`
	IncomeStartYear int       `json:"income_start_year"`
	ExpenseID       uuid.UUID `json:"expense_id"`
	ExpenseAmount   string    `json:"expense_amount"`
	OneTimeYear     int       `json:"one_time_year"`
	OneTimeAmount   string    `json:"one_time_amount"`
}

func ApplyOverrides(input *retirement.Input, overrides []byte, incomes []Resource, expenses []Resource) error {
	if len(overrides) == 0 {
		return nil
	}
	var o Overrides
	if err := json.Unmarshal(overrides, &o); err != nil {
		return err
	}

	if o.AnnualReturn != "" {
		value, err := decimal.NewFromString(o.AnnualReturn)
		if err != nil {
			return fmt.Errorf("annual return: %w", err)
		}
		input.Assumptions.AnnualReturn = value
	}
	if o.Inflation != "" {
		value, err := decimal.NewFromString(o.Inflation)
		if err != nil {
			return fmt.Errorf("inflation: %w", err)
		}
		input.Assumptions.Inflation = value
	}

	if o.PersonID != uuid.Nil && o.RetirementYear > 0 {
		for i := range input.People {
			if input.People[i].ID == o.PersonID.String() {
				input.People[i].RetirementYear = o.RetirementYear
			}
		}
		for i, inc := range input.Incomes {
			if inc.Kind == "employment" && inc.OwnerID == o.PersonID.String() {
				input.Incomes[i].EndYear = o.RetirementYear
			}
		}
	}
	if o.IncomeID != uuid.Nil && o.IncomeStartYear > 0 {
		for i, inc := range input.Incomes {
			if inc.ID == o.IncomeID.String() {
				input.Incomes[i].StartYear = o.IncomeStartYear
			}
		}
	}
	if o.ExpenseID != uuid.Nil && o.ExpenseAmount != "" {
		value, err := decimal.NewFromString(o.ExpenseAmount)
		if err != nil {
			return fmt.Errorf("expense amount: %w", err)
		}
		for i, exp := range input.Expenses {
			if exp.ID == o.ExpenseID.String() {
				input.Expenses[i].Amount = value
			}
		}
	}
	_ = incomes
	_ = expenses
	if o.OneTimeYear > 0 && o.OneTimeAmount != "" {
		value, err := decimal.NewFromString(o.OneTimeAmount)
		if err != nil {
			return fmt.Errorf("one-time amount: %w", err)
		}
		input.Expenses = append(input.Expenses, retirement.Expense{
			Name: "One Time Override", Amount: value,
			StartYear: o.OneTimeYear, EndYear: o.OneTimeYear,
		})
	}
	return nil
}
