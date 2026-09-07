package estate

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

const (
	ChecklistPass          = "pass"
	ChecklistAttention     = "attention"
	ChecklistNotApplicable = "not_applicable"
)

type ChecklistItem struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

type ReadinessDocument struct {
	DocumentType string
	Status       string
}

type ReadinessContact struct {
	Role string
}

type ReadinessInput struct {
	Plan         Input
	Documents    []ReadinessDocument
	Contacts     []ReadinessContact
	HasScenario  bool
	ScenarioName string
}

func EvaluateReadiness(in ReadinessInput) []ChecklistItem {
	items := []ChecklistItem{
		documentChecklist("signed_will", "Signed will inventory", in.Documents, isWill, "No household-level signed will is recorded. Documents are not linked to individual adults, so this does not establish per-adult coverage or validity."),
		contactChecklist("executor", "Executor contact", in.Contacts, "executor", "No executor contact is recorded."),
		guardianChecklist(in.Plan.Beneficiaries, in.Contacts),
		documentChecklist("financial_poa", "Financial power of attorney inventory", in.Documents, isFinancialPOA, "No household-level signed financial power of attorney is recorded. Documents are not linked to individual adults, so this does not establish per-adult coverage or validity."),
		documentChecklist("healthcare_directive", "Healthcare directive inventory", in.Documents, isHealthcareDirective, "No household-level signed healthcare directive is recorded. Documents are not linked to individual adults, so this does not establish per-adult coverage or validity."),
		designationChecklist("asset_primary", "Asset primary designations", applicableAssetDesignations(in.Plan.Assets), TierPrimary),
		designationChecklist("asset_contingent", "Asset contingent designations", applicableAssetDesignations(in.Plan.Assets), TierContingent),
		designationChecklist("insurance_primary", "Insurance primary designations", applicableInsuranceDesignations(in.Plan.Insurance), TierPrimary),
		designationChecklist("insurance_contingent", "Insurance contingent designations", applicableInsuranceDesignations(in.Plan.Insurance), TierContingent),
		residuaryChecklist(in.Plan.Residuary),
		liquidityChecklist(in),
	}
	return items
}

func documentChecklist(key, label string, documents []ReadinessDocument, match func(string) bool, missing string) ChecklistItem {
	for _, document := range documents {
		if match(document.DocumentType) && document.Status == "signed" {
			return ChecklistItem{Key: key, Label: label, Status: ChecklistPass, Reason: "A household-level signed document is recorded; professional review and per-adult coverage are not verified."}
		}
	}
	return ChecklistItem{Key: key, Label: label, Status: ChecklistAttention, Reason: missing}
}

func contactChecklist(key, label string, contacts []ReadinessContact, role, missing string) ChecklistItem {
	for _, contact := range contacts {
		if strings.EqualFold(strings.TrimSpace(contact.Role), role) {
			return ChecklistItem{Key: key, Label: label, Status: ChecklistPass, Reason: "A household contact with role " + role + " is recorded."}
		}
	}
	return ChecklistItem{Key: key, Label: label, Status: ChecklistAttention, Reason: missing}
}

func guardianChecklist(beneficiaries []Beneficiary, contacts []ReadinessContact) ChecklistItem {
	dependent := false
	for _, beneficiary := range beneficiaries {
		dependent = dependent || beneficiary.Dependent
	}
	if !dependent {
		return ChecklistItem{Key: "guardian", Label: "Guardian contact", Status: ChecklistNotApplicable, Reason: "No beneficiary is marked dependent."}
	}
	return contactChecklist("guardian", "Guardian contact", contacts, "guardian", "A dependent beneficiary is recorded, but no guardian contact is recorded.")
}

type namedDesignations struct {
	name         string
	designations []Designation
}

func applicableAssetDesignations(assets []Asset) []namedDesignations {
	out := []namedDesignations{}
	for _, asset := range assets {
		if asset.TransferMethod == TransferDesignate {
			out = append(out, namedDesignations{name: asset.Name, designations: asset.Designations})
		}
	}
	return out
}

func applicableInsuranceDesignations(policies []Insurance) []namedDesignations {
	out := []namedDesignations{}
	for _, policy := range policies {
		if !policy.InEstate {
			out = append(out, namedDesignations{name: policy.PolicyName, designations: policy.Designations})
		}
	}
	return out
}

func designationChecklist(key, label string, records []namedDesignations, tier string) ChecklistItem {
	if len(records) == 0 {
		return ChecklistItem{Key: key, Label: label, Status: ChecklistNotApplicable, Reason: "No applicable records use direct beneficiary designations."}
	}
	missing := []string{}
	for _, record := range records {
		total := decimal.Zero
		count := 0
		for _, designation := range record.designations {
			if designation.Tier == tier {
				total = total.Add(designation.SharePercent)
				count++
			}
		}
		if count == 0 || !total.Equal(decimal.NewFromInt(1)) {
			missing = append(missing, record.name)
		}
	}
	if len(missing) > 0 {
		return ChecklistItem{Key: key, Label: label, Status: ChecklistAttention, Reason: fmt.Sprintf("%s tier is missing or does not total 100%% for: %s.", designationTierLabel(tier), strings.Join(missing, ", "))}
	}
	return ChecklistItem{Key: key, Label: label, Status: ChecklistPass, Reason: designationTierLabel(tier) + " shares total 100% for every applicable record."}
}

func designationTierLabel(tier string) string {
	if tier == TierPrimary {
		return "Primary"
	}
	return "Contingent"
}

func residuaryChecklist(shares []ResiduaryShare) ChecklistItem {
	total := decimal.Zero
	for _, share := range shares {
		total = total.Add(share.SharePercent)
	}
	if len(shares) > 0 && total.Equal(decimal.NewFromInt(1)) {
		return ChecklistItem{Key: "residuary", Label: "Residuary shares", Status: ChecklistPass, Reason: "Recorded residuary shares total 100%."}
	}
	return ChecklistItem{Key: "residuary", Label: "Residuary shares", Status: ChecklistAttention, Reason: "Recorded residuary shares must total exactly 100%."}
}

func liquidityChecklist(in ReadinessInput) ChecklistItem {
	if !in.HasScenario {
		return ChecklistItem{Key: "liquidity", Label: "Probate liquidity", Status: ChecklistAttention, Reason: "Add a death scenario to compare liquid probate assets with entered obligations."}
	}
	liquid := decimal.Zero
	for _, asset := range in.Plan.Assets {
		if asset.TransferMethod == TransferProbate && asset.Liquid {
			liquid = liquid.Add(asset.Value)
		}
	}
	for _, policy := range in.Plan.Insurance {
		if policy.InEstate {
			liquid = liquid.Add(policy.Benefit)
		}
	}
	legalCost := in.Plan.LegalCost
	if legalCost.IsZero() && in.Plan.ProbateCost.IsPositive() {
		legalCost = in.Plan.ProbateCost
	}
	obligations := in.Plan.FuneralCost.Add(legalCost)
	if in.Plan.PayDebts {
		obligations = obligations.Add(in.Plan.Debt)
	}
	name := strings.TrimSpace(in.ScenarioName)
	if name == "" {
		name = "selected scenario"
	}
	if liquid.GreaterThanOrEqual(obligations) {
		return ChecklistItem{Key: "liquidity", Label: "Probate liquidity", Status: ChecklistPass, Reason: fmt.Sprintf("Liquid probate assets cover entered obligations for %s by %s.", name, liquid.Sub(obligations).StringFixed(2))}
	}
	return ChecklistItem{Key: "liquidity", Label: "Probate liquidity", Status: ChecklistAttention, Reason: fmt.Sprintf("Liquid probate assets are short by %s for %s.", obligations.Sub(liquid).StringFixed(2), name)}
}

func normalizedDocumentType(value string) string { return strings.ToLower(strings.TrimSpace(value)) }
func isWill(value string) bool {
	value = normalizedDocumentType(value)
	return (value == "will" || strings.Contains(value, "last will")) && !strings.Contains(value, "living will")
}
func isFinancialPOA(value string) bool {
	value = normalizedDocumentType(value)
	return !strings.Contains(value, "health") && (strings.Contains(value, "financial power of attorney") || strings.Contains(value, "durable power of attorney") || strings.Contains(value, "financial poa"))
}
func isHealthcareDirective(value string) bool {
	value = normalizedDocumentType(value)
	return strings.Contains(value, "healthcare directive") || strings.Contains(value, "health care directive") || strings.Contains(value, "advance directive") || strings.Contains(value, "living will")
}
