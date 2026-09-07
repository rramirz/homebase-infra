// Package retirement is pure deterministic nominal retirement projection logic.
package retirement

import (
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
)

var zero = decimal.Zero

type Person struct {
	ID, Name                             string
	BirthYear, RetirementYear, DeathYear int
}
type Account struct {
	Name, Kind                                string
	Balance, AnnualReturn, AnnualContribution decimal.Decimal
	Owner                                     string
}
type Income struct {
	ID, Name, Owner, OwnerID, Kind string
	Amount                         decimal.Decimal
	StartYear, EndYear             int
	Inflation                      decimal.Decimal
}
type Expense struct {
	ID, Name, Kind     string
	Amount             decimal.Decimal
	StartYear, EndYear int
	Inflation          decimal.Decimal
}
type Liability struct {
	Name                               string
	Balance, AnnualRate, AnnualPayment decimal.Decimal
}
type Assumptions struct {
	AsOfYear, EndYear       int
	AnnualReturn, Inflation decimal.Decimal
}
type Input struct {
	Assumptions Assumptions
	People      []Person
	Accounts    []Account
	Incomes     []Income
	Expenses    []Expense
	Liabilities []Liability
}
type Event struct {
	Year       int
	Kind, Name string
}
type Year struct {
	Year                                                                        int
	Income, Expenses, DebtPaid, NetCashFlow, Growth, EndingPortfolio, Shortfall decimal.Decimal
	StartingPortfolio, Contributions, Withdrawals                               decimal.Decimal
	Cash, Taxable, TaxDeferred, Roth                                            decimal.Decimal
	LiabilityBalance                                                            decimal.Decimal
	Ages                                                                        map[string]int
	Events                                                                      []Event
}
type Projection struct {
	Years         []Year
	DepletionYear int
}

func inflated(amount, rate decimal.Decimal, offset int) decimal.Decimal {
	if offset <= 0 {
		return amount
	}
	return amount.Mul(decimal.NewFromInt(1).Add(rate).Pow(decimal.NewFromInt(int64(offset))))
}
func active(start, end, year int) bool { return year >= start && (end == 0 || year <= end) }
func alive(p Person, year int) bool    { return p.DeathYear == 0 || year <= p.DeathYear }
func (in Input) Validate() error {
	if in.Assumptions.AsOfYear < 1900 || in.Assumptions.EndYear < in.Assumptions.AsOfYear {
		return errors.New("invalid projection years")
	}
	for _, a := range in.Accounts {
		if a.Kind != "cash" && a.Kind != "taxable" && a.Kind != "tax_deferred" && a.Kind != "roth" {
			return fmt.Errorf("invalid account kind %q", a.Kind)
		}
		if a.Balance.IsNegative() || a.AnnualContribution.IsNegative() {
			return errors.New("negative account value")
		}
	}
	for _, x := range in.Incomes {
		if x.Amount.IsNegative() || x.StartYear == 0 || (x.EndYear != 0 && x.EndYear < x.StartYear) {
			return errors.New("invalid income")
		}
	}
	for _, x := range in.Expenses {
		if x.Amount.IsNegative() || x.StartYear == 0 || (x.EndYear != 0 && x.EndYear < x.StartYear) {
			return errors.New("invalid expense")
		}
	}
	for _, l := range in.Liabilities {
		if l.Balance.IsNegative() || l.AnnualPayment.IsNegative() {
			return errors.New("invalid liability")
		}
	}
	return nil
}
func Project(in Input) (Projection, error) {
	if err := in.Validate(); err != nil {
		return Projection{}, err
	}
	balances := map[string]decimal.Decimal{"cash": zero, "taxable": zero, "tax_deferred": zero, "roth": zero}
	returns := map[string]decimal.Decimal{}
	contributions := map[string]decimal.Decimal{}
	for _, a := range in.Accounts {
		balances[a.Kind] = balances[a.Kind].Add(a.Balance)
		returns[a.Kind] = a.AnnualReturn
		contributions[a.Kind] = contributions[a.Kind].Add(a.AnnualContribution)
	}
	liabilities := append([]Liability(nil), in.Liabilities...)
	out := Projection{}
	for year := in.Assumptions.AsOfYear; year <= in.Assumptions.EndYear; year++ {
		starting := balances["cash"].Add(balances["taxable"]).Add(balances["tax_deferred"]).Add(balances["roth"])
		income, expenses, debt := zero, zero, zero
		ages := map[string]int{}
		events := []Event{}
		for _, p := range in.People {
			ages[p.Name] = year - p.BirthYear
			if p.RetirementYear == year {
				events = append(events, Event{year, "retirement", p.Name})
			}
			if p.DeathYear == year {
				events = append(events, Event{year, "death", p.Name})
			}
		}
		for _, x := range in.Incomes {
			if active(x.StartYear, x.EndYear, year) && ownerAlive(in.People, x.Owner, year) {
				income = income.Add(inflated(x.Amount, x.Inflation, year-x.StartYear))
				if x.StartYear == year {
					events = append(events, Event{year, "income starts", x.Name})
				}
			}
		}
		for _, x := range in.Expenses {
			if active(x.StartYear, x.EndYear, year) {
				expenses = expenses.Add(inflated(x.Amount, x.Inflation, year-x.StartYear))
				if x.StartYear == year {
					events = append(events, Event{year, "expense starts", x.Name})
				}
			}
		}
		for i := range liabilities {
			l := &liabilities[i]
			if l.Balance.GreaterThan(zero) {
				l.Balance = l.Balance.Mul(decimal.NewFromInt(1).Add(l.AnnualRate))
				p := decimal.Min(l.AnnualPayment, l.Balance)
				l.Balance = l.Balance.Sub(p)
				debt = debt.Add(p)
				if l.Balance.IsZero() {
					events = append(events, Event{year, "liability paid", l.Name})
				}
			}
		}
		net := income.Sub(expenses).Sub(debt)
		short, withdrawals := zero, zero
		if net.GreaterThanOrEqual(zero) {
			balances["cash"] = balances["cash"].Add(net)
		} else {
			need := net.Neg()
			for _, k := range []string{"cash", "taxable", "tax_deferred", "roth"} {
				take := decimal.Min(balances[k], need)
				balances[k] = balances[k].Sub(take)
				need = need.Sub(take)
				withdrawals = withdrawals.Add(take)
			}
			short = need
			if need.GreaterThan(zero) && out.DepletionYear == 0 {
				out.DepletionYear = year
			}
		}
		contributed := zero
		for k, c := range contributions {
			balances[k] = balances[k].Add(c)
			contributed = contributed.Add(c)
		}
		growth := zero
		for _, k := range []string{"cash", "taxable", "tax_deferred", "roth"} {
			rate := returns[k]
			if rate.IsZero() {
				rate = in.Assumptions.AnnualReturn
			}
			g := balances[k].Mul(rate)
			balances[k] = balances[k].Add(g)
			growth = growth.Add(g)
		}
		ending := balances["cash"].Add(balances["taxable"]).Add(balances["tax_deferred"]).Add(balances["roth"])
		lb := zero
		for _, l := range liabilities {
			lb = lb.Add(l.Balance)
		}
		out.Years = append(out.Years, Year{Year: year, Income: income, Expenses: expenses, DebtPaid: debt, NetCashFlow: net, Growth: growth, EndingPortfolio: ending, Shortfall: short, StartingPortfolio: starting, Contributions: contributed, Withdrawals: withdrawals, Cash: balances["cash"], Taxable: balances["taxable"], TaxDeferred: balances["tax_deferred"], Roth: balances["roth"], LiabilityBalance: lb, Ages: ages, Events: events})
	}
	return out, nil
}
func ownerAlive(people []Person, owner string, year int) bool {
	if owner == "" {
		return true
	}
	for _, p := range people {
		if p.Name == owner {
			return alive(p, year)
		}
	}
	return true
}
