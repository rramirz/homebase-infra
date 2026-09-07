package retirement

import (
	"github.com/shopspring/decimal"
	"testing"
)

func d(s string) decimal.Decimal { return decimal.RequireFromString(s) }
func project(t *testing.T, in Input) Projection {
	t.Helper()
	p, e := Project(in)
	if e != nil {
		t.Fatal(e)
	}
	return p
}
func TestZeroAssetsDepletes(t *testing.T) {
	p := project(t, Input{Assumptions: Assumptions{2025, 2025, zero, zero}, Expenses: []Expense{{Name: "rent", Amount: d("100"), StartYear: 2025}}})
	if p.DepletionYear != 2025 || !p.Years[0].Shortfall.Equal(d("100")) {
		t.Fatal(p)
	}
}
func TestCoupleRetirementSSAndDeath(t *testing.T) {
	p := project(t, Input{Assumptions: Assumptions{2025, 2030, zero, zero}, People: []Person{{Name: "A", BirthYear: 1970, RetirementYear: 2025, DeathYear: 2028}, {Name: "B", BirthYear: 1980, RetirementYear: 2027}}, Incomes: []Income{{Name: "A ss", Owner: "A", Kind: "social_security", Amount: d("10"), StartYear: 2026}, {Name: "B job", Owner: "B", Kind: "employment", Amount: d("5"), StartYear: 2025}}})
	if !p.Years[1].Income.Equal(d("15")) || !p.Years[4].Income.Equal(d("5")) || p.Years[0].Ages["A"] != 55 {
		t.Fatal(p.Years)
	}
}
func TestAccountContributionReturnAndWithdrawalOrder(t *testing.T) {
	p := project(t, Input{Assumptions: Assumptions{2025, 2025, d("0.1"), zero}, Accounts: []Account{{Kind: "cash", Balance: d("10")}, {Kind: "taxable", Balance: d("20"), AnnualReturn: d("0.2"), AnnualContribution: d("5")}}, Expenses: []Expense{{Amount: d("15"), StartYear: 2025}}})
	if !p.Years[0].Cash.IsZero() || !p.Years[0].Taxable.Equal(d("24")) {
		t.Fatal(p.Years[0])
	}
}
func TestExpenseCategoriesOneTimeInflationAndLiability(t *testing.T) {
	p := project(t, Input{Assumptions: Assumptions{2025, 2026, zero, zero}, Accounts: []Account{{Kind: "cash", Balance: d("1000")}}, Expenses: []Expense{{Kind: "essential", Amount: d("100"), StartYear: 2025, EndYear: 2026, Inflation: d(".1")}, {Kind: "one_time", Amount: d("50"), StartYear: 2026, EndYear: 2026}}, Liabilities: []Liability{{Balance: d("100"), AnnualPayment: d("60")}}})
	if !p.Years[1].Expenses.Equal(d("160")) || !p.Years[1].DebtPaid.Equal(d("40")) || !p.Years[1].LiabilityBalance.IsZero() {
		t.Fatal(p.Years[1])
	}
}
func TestInputValidationAndDeterminism(t *testing.T) {
	if _, e := Project(Input{Assumptions: Assumptions{2026, 2025, zero, zero}}); e == nil {
		t.Fatal("expected validation")
	}
	in := Input{Assumptions: Assumptions{2025, 2026, d(".05"), zero}, Accounts: []Account{{Kind: "roth", Balance: d("100")}}}
	a := project(t, in)
	b := project(t, in)
	if !a.Years[1].EndingPortfolio.Equal(b.Years[1].EndingPortfolio) {
		t.Fatal("nondeterministic")
	}
	in.Assumptions.AnnualReturn = d(".1")
	if project(t, in).Years[1].EndingPortfolio.Equal(a.Years[1].EndingPortfolio) {
		t.Fatal("scenario did not diverge")
	}
}
