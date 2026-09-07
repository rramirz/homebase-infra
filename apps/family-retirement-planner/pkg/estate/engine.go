package estate

import (
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	StageInsurance    = "insurance"
	StageWROS         = "wros"
	StageDesignation  = "designation"
	StageTrust        = "trust"
	StageProbate      = "probate"
	StageObligations  = "obligations"
	StageResiduary    = "residuary"
	StageWarnings     = "warnings"
	TransferProbate   = "probate"
	TransferWROS      = "wros"
	TransferDesignate = "designation"
	TransferTrust     = "trust"
	TierPrimary       = "primary"
	TierContingent    = "contingent"
)

type Event struct {
	Stage  string          `json:"stage"`
	Source string          `json:"source"`
	Target string          `json:"target"`
	Amount decimal.Decimal `json:"amount"`
	Note   string          `json:"note"`
}

type Beneficiary struct {
	ID           uuid.UUID
	Name         string
	Relationship string
	Dependent    bool
}

type Owner struct {
	Name string
}

type Designation struct {
	BeneficiaryID uuid.UUID
	SharePercent  decimal.Decimal
	Tier          string
}

type Asset struct {
	ID             uuid.UUID
	Name           string
	Value          decimal.Decimal
	Liquid         bool
	TransferMethod string
	TrustID        uuid.UUID
	Owners         []Owner
	Designations   []Designation
}

type Insurance struct {
	ID           uuid.UUID
	PolicyName   string
	Benefit      decimal.Decimal
	InEstate     bool
	Designations []Designation
}

type Trust struct {
	ID           uuid.UUID
	Name         string
	Designations []Designation
}

type ResiduaryShare struct {
	BeneficiaryID uuid.UUID
	SharePercent  decimal.Decimal
}

type Input struct {
	AsOfYear      int
	DeathYear     int
	DecedentName  string
	Beneficiaries []Beneficiary
	Assets        []Asset
	Insurance     []Insurance
	Trusts        []Trust
	Residuary     []ResiduaryShare
	Debt          decimal.Decimal
	FuneralCost   decimal.Decimal
	LegalCost     decimal.Decimal
	PayDebts      bool
	// ProbateCost supports reading pre-MVP input snapshots. New code leaves it zero.
	ProbateCost decimal.Decimal
}

type ObligationDetail struct {
	Kind     string          `json:"kind"`
	Amount   decimal.Decimal `json:"amount"`
	Included bool            `json:"included"`
}

type DesignationTierStatus struct {
	Source   string          `json:"source"`
	Tier     string          `json:"tier"`
	Total    decimal.Decimal `json:"total"`
	Recorded bool            `json:"recorded"`
	Complete bool            `json:"complete"`
}

type Result struct {
	AsOfYear           int                     `json:"as_of_year"`
	DeathYear          int                     `json:"death_year"`
	Events             []Event                 `json:"events"`
	GrossEstate        decimal.Decimal         `json:"gross_estate"`
	ProbateEstate      decimal.Decimal         `json:"probate_estate"`
	LiquidProbate      decimal.Decimal         `json:"liquid_probate"`
	Obligations        decimal.Decimal         `json:"obligations"`
	ObligationDetails  []ObligationDetail      `json:"obligation_details"`
	DesignationTiers   []DesignationTierStatus `json:"designation_tiers"`
	Readiness          []ChecklistItem         `json:"readiness,omitempty"`
	NetEstate          decimal.Decimal         `json:"net_estate"`
	LiquidityShortfall decimal.Decimal         `json:"liquidity_shortfall"`
	LiquidityWarning   bool                    `json:"liquidity_warning"`
	Insolvent          bool                    `json:"insolvent"`
	Valid              bool                    `json:"valid"`
	Errors             []string                `json:"errors,omitempty"`
}

func Run(in Input) (Result, error) {
	res := Result{Valid: true, AsOfYear: in.AsOfYear, DeathYear: in.DeathYear}
	beneficiaries, trusts := validate(in, &res)
	if !res.Valid {
		return res, nil
	}

	// Stage 1: life insurance pays named beneficiaries directly or enters probate.
	for _, policy := range sortedInsurance(in.Insurance) {
		if policy.InEstate {
			res.ProbateEstate = res.ProbateEstate.Add(policy.Benefit)
			res.LiquidProbate = res.LiquidProbate.Add(policy.Benefit)
			res.Events = append(res.Events, Event{Stage: StageInsurance, Source: policy.PolicyName, Target: "probate estate", Amount: policy.Benefit, Note: "Policy payable to the estate"})
			continue
		}
		appendDistributions(&res, StageInsurance, policy.PolicyName, policy.Benefit, primaryDesignations(policy.Designations), beneficiaries, "Primary named-beneficiary insurance payout")
	}

	// Stages 2-5 classify assets in stable stage order: WROS, then direct
	// designations, then trust transfers, then remaining probate assets. Each
	// pass is internally sorted by asset UUID so stage order never depends on
	// input ordering or UUID values.
	for _, method := range []string{TransferWROS, TransferDesignate, TransferTrust, TransferProbate} {
		classifyAssets(&res, in, beneficiaries, trusts, method)
	}
	res.GrossEstate = res.GrossEstate.Add(insuranceTotal(in.Insurance))

	// Stage 6: explicit user-entered obligations reduce probate value. No law or tax rules are inferred.
	legalCost := in.LegalCost
	if legalCost.IsZero() && in.ProbateCost.IsPositive() {
		legalCost = in.ProbateCost
	}
	res.ObligationDetails = []ObligationDetail{
		{Kind: "funeral", Amount: in.FuneralCost, Included: true},
		{Kind: "legal_administration", Amount: legalCost, Included: true},
		{Kind: "debts", Amount: in.Debt, Included: in.PayDebts},
	}
	res.Obligations = in.FuneralCost.Add(legalCost)
	if in.PayDebts {
		res.Obligations = res.Obligations.Add(in.Debt)
	}
	if in.FuneralCost.IsPositive() {
		res.Events = append(res.Events, Event{Stage: StageObligations, Source: "Entered funeral costs", Target: "probate estate", Amount: in.FuneralCost.Neg(), Note: "User-entered estimate"})
	}
	if legalCost.IsPositive() {
		res.Events = append(res.Events, Event{Stage: StageObligations, Source: "Entered legal and administration costs", Target: "probate estate", Amount: legalCost.Neg(), Note: "User-entered estimate; not a jurisdiction calculation"})
	}
	if in.PayDebts && in.Debt.IsPositive() {
		res.Events = append(res.Events, Event{Stage: StageObligations, Source: "Debts and final obligations", Target: "probate estate", Amount: in.Debt.Neg(), Note: "User-entered estimate; priority and enforceability are not modeled"})
	}
	res.NetEstate = res.ProbateEstate.Sub(res.Obligations)
	if res.NetEstate.IsNegative() {
		res.Insolvent = true
		res.NetEstate = decimal.Zero
	}

	// Stage 7: only the non-negative probate residue is allocated.
	for _, share := range sortedResiduary(in.Residuary) {
		res.Events = append(res.Events, Event{Stage: StageResiduary, Source: "probate residue", Target: beneficiaries[share.BeneficiaryID], Amount: res.NetEstate.Mul(share.SharePercent), Note: "Modeled residuary allocation"})
	}

	// Stage 8: liquidity and solvency are explicit outputs, not hidden legal conclusions.
	if res.LiquidProbate.LessThan(res.Obligations) {
		res.LiquidityWarning = true
		res.LiquidityShortfall = res.Obligations.Sub(res.LiquidProbate)
		res.Events = append(res.Events, Event{Stage: StageWarnings, Source: "liquid probate assets", Target: "obligations", Amount: res.LiquidityShortfall.Neg(), Note: "Liquid assets are below entered obligations; asset sale timing and legal priority are not modeled"})
	}
	if res.Insolvent {
		res.Events = append(res.Events, Event{Stage: StageWarnings, Source: "probate estate", Target: "obligations", Amount: res.Obligations.Sub(res.ProbateEstate).Neg(), Note: "Entered obligations exceed modeled probate assets"})
	}
	return res, nil
}

// classifyAssets adds every asset transferred by method to the gross estate and
// emits its stage event. Assets are processed in UUID order within the method
// so stage order stays stable regardless of input ordering.
func classifyAssets(res *Result, in Input, beneficiaries map[uuid.UUID]string, trusts map[uuid.UUID]Trust, method string) {
	for _, asset := range sortedAssets(in.Assets) {
		if asset.TransferMethod != method {
			continue
		}
		res.GrossEstate = res.GrossEstate.Add(asset.Value)
		switch method {
		case TransferWROS:
			target, err := survivingOwners(asset.Owners, in.DecedentName)
			if err != nil {
				// Unreachable: validate rejects WROS assets that do not record
				// the decedent as an owner or a distinct surviving owner.
				res.Errors = append(res.Errors, "WROS asset "+asset.Name+" "+err.Error())
				continue
			}
			res.Events = append(res.Events, Event{Stage: StageWROS, Source: asset.Name, Target: target, Amount: asset.Value, Note: "Transfers to recorded surviving joint owner; title and law still require professional review"})
		case TransferDesignate:
			appendDistributions(res, StageDesignation, asset.Name, asset.Value, primaryDesignations(asset.Designations), beneficiaries, "Recorded primary TOD/POD or beneficiary transfer")
		case TransferTrust:
			trust := trusts[asset.TrustID]
			appendDistributions(res, StageTrust, asset.Name+" via "+trust.Name, asset.Value, primaryDesignations(trust.Designations), beneficiaries, "Modeled primary trust allocation; contingent allocations are readiness metadata only")
		case TransferProbate:
			res.ProbateEstate = res.ProbateEstate.Add(asset.Value)
			if asset.Liquid {
				res.LiquidProbate = res.LiquidProbate.Add(asset.Value)
			}
			res.Events = append(res.Events, Event{Stage: StageProbate, Source: asset.Name, Target: "probate estate", Amount: asset.Value, Note: liquidityNote(asset.Liquid)})
		}
	}
}

func validate(in Input, res *Result) (map[uuid.UUID]string, map[uuid.UUID]Trust) {
	if in.AsOfYear < 0 || in.DeathYear < 0 || (in.AsOfYear > 0 && in.DeathYear > 0 && in.DeathYear < in.AsOfYear) {
		res.Errors = append(res.Errors, "death year must be on or after the as-of year")
	}
	beneficiaries := make(map[uuid.UUID]string, len(in.Beneficiaries))
	for _, b := range in.Beneficiaries {
		if b.ID == uuid.Nil || strings.TrimSpace(b.Name) == "" {
			res.Errors = append(res.Errors, "beneficiaries require an ID and name")
			continue
		}
		if _, exists := beneficiaries[b.ID]; exists {
			res.Errors = append(res.Errors, "beneficiary IDs must be unique")
		}
		beneficiaries[b.ID] = b.Name
	}
	trusts := make(map[uuid.UUID]Trust, len(in.Trusts))
	for _, trust := range in.Trusts {
		if trust.ID == uuid.Nil || strings.TrimSpace(trust.Name) == "" {
			res.Errors = append(res.Errors, "trusts require an ID and name")
			continue
		}
		validateDesignationTiers("trust "+trust.Name, trust.Designations, beneficiaries, true, res)
		trusts[trust.ID] = trust
	}
	for _, asset := range in.Assets {
		if strings.TrimSpace(asset.Name) == "" || asset.Value.IsNegative() {
			res.Errors = append(res.Errors, "assets require a name and non-negative value")
		}
		switch asset.TransferMethod {
		case TransferProbate:
		case TransferWROS:
			if _, err := survivingOwners(asset.Owners, in.DecedentName); err != nil {
				res.Errors = append(res.Errors, "WROS asset "+asset.Name+" "+err.Error())
			}
		case TransferDesignate:
			validateDesignationTiers("asset "+asset.Name, asset.Designations, beneficiaries, true, res)
		case TransferTrust:
			if _, ok := trusts[asset.TrustID]; !ok {
				res.Errors = append(res.Errors, "asset "+asset.Name+" references an unknown trust")
			}
		default:
			res.Errors = append(res.Errors, "asset "+asset.Name+" has an invalid transfer method")
		}
	}
	for _, policy := range in.Insurance {
		if strings.TrimSpace(policy.PolicyName) == "" || policy.Benefit.IsNegative() {
			res.Errors = append(res.Errors, "insurance requires a policy name and non-negative benefit")
		}
		if !policy.InEstate {
			validateDesignationTiers("insurance "+policy.PolicyName, policy.Designations, beneficiaries, true, res)
		}
	}
	if in.Debt.IsNegative() || in.FuneralCost.IsNegative() || in.LegalCost.IsNegative() || in.ProbateCost.IsNegative() {
		res.Errors = append(res.Errors, "debts, funeral costs, and legal costs cannot be negative")
	}
	residuary := make([]Designation, 0, len(in.Residuary))
	for _, share := range in.Residuary {
		residuary = append(residuary, Designation{BeneficiaryID: share.BeneficiaryID, SharePercent: share.SharePercent})
	}
	if err := validateShares("residuary plan", residuary, beneficiaries); err != nil {
		res.Errors = append(res.Errors, err.Error())
	}
	res.Valid = len(res.Errors) == 0
	return beneficiaries, trusts
}

func validateDesignationTiers(label string, shares []Designation, beneficiaries map[uuid.UUID]string, requirePrimary bool, res *Result) {
	byTier := map[string][]Designation{TierPrimary: {}, TierContingent: {}}
	for _, share := range shares {
		if share.Tier != TierPrimary && share.Tier != TierContingent {
			res.Errors = append(res.Errors, label+" has an invalid designation tier")
			continue
		}
		byTier[share.Tier] = append(byTier[share.Tier], share)
	}
	for _, tier := range []string{TierPrimary, TierContingent} {
		tierShares := byTier[tier]
		total := designationTotal(tierShares)
		complete := len(tierShares) > 0 && total.Equal(decimal.NewFromInt(1))
		res.DesignationTiers = append(res.DesignationTiers, DesignationTierStatus{Source: label, Tier: tier, Total: total, Recorded: len(tierShares) > 0, Complete: complete})
		if len(tierShares) == 0 {
			if tier == TierPrimary && requirePrimary {
				res.Errors = append(res.Errors, label+" requires primary beneficiary shares")
			}
			continue
		}
		if err := validateShares(label+" "+tier+" tier", tierShares, beneficiaries); err != nil {
			res.Errors = append(res.Errors, err.Error())
		}
	}
}

func designationTotal(shares []Designation) decimal.Decimal {
	total := decimal.Zero
	for _, share := range shares {
		total = total.Add(share.SharePercent)
	}
	return total
}

func primaryDesignations(shares []Designation) []Designation {
	out := make([]Designation, 0, len(shares))
	for _, share := range shares {
		if share.Tier == TierPrimary {
			out = append(out, share)
		}
	}
	return out
}

func validateShares(label string, shares []Designation, beneficiaries map[uuid.UUID]string) error {
	if len(shares) == 0 {
		return fmt.Errorf("%s requires beneficiary shares", label)
	}
	total := decimal.Zero
	seen := map[uuid.UUID]bool{}
	for _, share := range shares {
		if _, ok := beneficiaries[share.BeneficiaryID]; !ok {
			return fmt.Errorf("%s references an unknown beneficiary", label)
		}
		if seen[share.BeneficiaryID] || !share.SharePercent.IsPositive() || share.SharePercent.GreaterThan(decimal.NewFromInt(1)) {
			return fmt.Errorf("%s has invalid beneficiary shares", label)
		}
		seen[share.BeneficiaryID] = true
		total = total.Add(share.SharePercent)
	}
	if !total.Equal(decimal.NewFromInt(1)) {
		return fmt.Errorf("%s shares must equal 100%%", label)
	}
	return nil
}

func appendDistributions(res *Result, stage, source string, amount decimal.Decimal, shares []Designation, beneficiaries map[uuid.UUID]string, note string) {
	for _, share := range sortedDesignations(shares) {
		res.Events = append(res.Events, Event{Stage: stage, Source: source, Target: beneficiaries[share.BeneficiaryID], Amount: amount.Mul(share.SharePercent), Note: note})
	}
}

// survivingOwners returns the case-insensitively distinct recorded owners other
// than the decedent. A joint-ownership-with-survivorship transfer requires the
// decedent to be a recorded owner AND at least one other recorded owner to
// survive; either missing condition is an error.
func survivingOwners(owners []Owner, decedent string) (string, error) {
	dec := strings.TrimSpace(decedent)
	hasDecedent := false
	names := make([]string, 0, len(owners))
	for _, owner := range owners {
		name := strings.TrimSpace(owner.Name)
		if name == "" {
			continue
		}
		if strings.EqualFold(name, dec) {
			hasDecedent = true
			continue
		}
		names = append(names, name)
	}
	if !hasDecedent {
		return "", fmt.Errorf("requires the decedent to be a recorded owner")
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "", fmt.Errorf("requires a surviving owner other than the decedent")
	}
	return strings.Join(names, ", "), nil
}

func liquidityNote(liquid bool) string {
	if liquid {
		return "Liquid asset enters modeled probate estate"
	}
	return "Non-liquid asset enters modeled probate estate"
}

func insuranceTotal(policies []Insurance) decimal.Decimal {
	total := decimal.Zero
	for _, policy := range policies {
		total = total.Add(policy.Benefit)
	}
	return total
}

func sortedAssets(items []Asset) []Asset {
	out := append([]Asset(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID.String() < out[j].ID.String() })
	return out
}

func sortedInsurance(items []Insurance) []Insurance {
	out := append([]Insurance(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID.String() < out[j].ID.String() })
	return out
}

func sortedDesignations(items []Designation) []Designation {
	out := append([]Designation(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].BeneficiaryID.String() < out[j].BeneficiaryID.String() })
	return out
}

func sortedResiduary(items []ResiduaryShare) []ResiduaryShare {
	out := append([]ResiduaryShare(nil), items...)
	sort.Slice(out, func(i, j int) bool { return out[i].BeneficiaryID.String() < out[j].BeneficiaryID.String() })
	return out
}
