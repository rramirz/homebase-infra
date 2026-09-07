package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/pkg/estate"
	"github.com/shopspring/decimal"
)

type EstateBeneficiary struct {
	ID           uuid.UUID
	Name         string
	Relationship string
	Dependent    bool
}

type EstateDesignation struct {
	ID              uuid.UUID
	ParentID        uuid.UUID
	BeneficiaryID   uuid.UUID
	BeneficiaryName string
	SharePercent    decimal.Decimal
	Tier            string
}

type EstateTrust struct {
	ID           uuid.UUID
	Name         string
	Notes        string
	Designations []EstateDesignation
}

type EstateOwner struct {
	ID      uuid.UUID
	AssetID uuid.UUID
	Name    string
}

type EstateAsset struct {
	ID             uuid.UUID
	Name           string
	Value          decimal.Decimal
	Liquid         bool
	TransferMethod string
	TrustID        uuid.UUID
	TrustName      string
	Owners         []EstateOwner
	Designations   []EstateDesignation
}

type EstateInsurance struct {
	ID           uuid.UUID
	PolicyName   string
	Benefit      decimal.Decimal
	InEstate     bool
	Designations []EstateDesignation
}

type EstateDocument struct {
	ID              uuid.UUID
	DocumentType    string
	StorageLocation string
	Status          string
	Notes           string
}

type EstateContact struct {
	ID          uuid.UUID
	Role        string
	Name        string
	ContactInfo string
	Notes       string
}

type EstateDigitalAsset struct {
	ID                         uuid.UUID
	ServiceName                string
	AccountIdentifier          string
	AccessInstructionsLocation string
	Notes                      string
}

type EstateResiduaryShare struct {
	ID              uuid.UUID
	BeneficiaryID   uuid.UUID
	BeneficiaryName string
	SharePercent    decimal.Decimal
}

type EstateScenario struct {
	ID           uuid.UUID
	Name         string
	DecedentName string
	AsOfYear     int
	DeathYear    int
	Debt         decimal.Decimal
	FuneralCost  decimal.Decimal
	LegalCost    decimal.Decimal
	PayDebts     bool
}

type EstateResult struct {
	ID           uuid.UUID
	ScenarioID   uuid.UUID
	ScenarioName string
	DecedentName string
	AsOfYear     int
	DeathYear    int
	Input        estate.Input
	Result       estate.Result
	CreatedAt    time.Time
}

type EstateData struct {
	Beneficiaries []EstateBeneficiary
	Trusts        []EstateTrust
	Assets        []EstateAsset
	Insurance     []EstateInsurance
	Documents     []EstateDocument
	Contacts      []EstateContact
	DigitalAssets []EstateDigitalAsset
	Residuary     []EstateResiduaryShare
	Scenarios     []EstateScenario
	Results       []EstateResult
	Readiness     []estate.ChecklistItem
}

func (r *Repo) EstateData(ctx context.Context, scope Scope) (EstateData, error) {
	var out EstateData
	loaders := []func() error{
		func() error { var err error; out.Beneficiaries, err = r.EstateBeneficiaries(ctx, scope); return err },
		func() error { var err error; out.Trusts, err = r.EstateTrusts(ctx, scope); return err },
		func() error { var err error; out.Assets, err = r.EstateAssets(ctx, scope); return err },
		func() error { var err error; out.Insurance, err = r.EstateInsurancePolicies(ctx, scope); return err },
		func() error { var err error; out.Documents, err = r.EstateDocuments(ctx, scope); return err },
		func() error { var err error; out.Contacts, err = r.EstateContacts(ctx, scope); return err },
		func() error { var err error; out.DigitalAssets, err = r.EstateDigitalAssets(ctx, scope); return err },
		func() error { var err error; out.Residuary, err = r.EstateResiduaryShares(ctx, scope); return err },
		func() error { var err error; out.Scenarios, err = r.EstateScenarios(ctx, scope); return err },
		func() error { var err error; out.Results, err = r.EstateResults(ctx, scope); return err },
	}
	for _, load := range loaders {
		if err := load(); err != nil {
			return EstateData{}, err
		}
	}
	var scenario EstateScenario
	if len(out.Scenarios) > 0 {
		scenario = out.Scenarios[0]
	}
	out.Readiness = estate.EvaluateReadiness(estateReadinessInput(out, scenario))
	return out, nil
}

func (r *Repo) EstateBeneficiaries(ctx context.Context, scope Scope) ([]EstateBeneficiary, error) {
	rows, err := r.db.Query(ctx, `SELECT id,name,relationship,dependent FROM estate_beneficiaries WHERE user_id=$1 AND household_id=$2 ORDER BY created_at,id`, scope.UserID, scope.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateBeneficiary{}
	for rows.Next() {
		var item EstateBeneficiary
		if err := rows.Scan(&item.ID, &item.Name, &item.Relationship, &item.Dependent); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repo) CreateEstateBeneficiary(ctx context.Context, scope Scope, item EstateBeneficiary) (uuid.UUID, error) {
	return r.insertScoped(ctx, scope, `INSERT INTO estate_beneficiaries(user_id,household_id,name,relationship,dependent) SELECT user_id,id,$3,$4,$5 FROM households WHERE id=$2 AND user_id=$1 AND archived_at IS NULL RETURNING id`, strings.TrimSpace(item.Name), strings.TrimSpace(item.Relationship), item.Dependent)
}

func (r *Repo) UpdateEstateBeneficiary(ctx context.Context, scope Scope, item EstateBeneficiary) error {
	return r.updateScoped(ctx, `UPDATE estate_beneficiaries SET name=$4,relationship=$5,dependent=$6,updated_at=now() WHERE id=$1 AND user_id=$2 AND household_id=$3`, item.ID, scope, strings.TrimSpace(item.Name), strings.TrimSpace(item.Relationship), item.Dependent)
}

func (r *Repo) DeleteEstateBeneficiary(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_beneficiaries", scope, id)
}

func (r *Repo) EstateTrusts(ctx context.Context, scope Scope) ([]EstateTrust, error) {
	rows, err := r.db.Query(ctx, `SELECT id,name,notes FROM estate_trusts WHERE user_id=$1 AND household_id=$2 ORDER BY created_at,id`, scope.UserID, scope.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateTrust{}
	for rows.Next() {
		var item EstateTrust
		if err := rows.Scan(&item.ID, &item.Name, &item.Notes); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Designations, err = r.estateDesignations(ctx, scope, "estate_trust_designations", "trust_id", out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Repo) CreateEstateTrust(ctx context.Context, scope Scope, item EstateTrust) (uuid.UUID, error) {
	return r.insertScoped(ctx, scope, `INSERT INTO estate_trusts(user_id,household_id,name,notes) SELECT user_id,id,$3,$4 FROM households WHERE id=$2 AND user_id=$1 AND archived_at IS NULL RETURNING id`, strings.TrimSpace(item.Name), strings.TrimSpace(item.Notes))
}

func (r *Repo) UpdateEstateTrust(ctx context.Context, scope Scope, item EstateTrust) error {
	return r.updateScoped(ctx, `UPDATE estate_trusts SET name=$4,notes=$5,updated_at=now() WHERE id=$1 AND user_id=$2 AND household_id=$3`, item.ID, scope, strings.TrimSpace(item.Name), strings.TrimSpace(item.Notes))
}

func (r *Repo) DeleteEstateTrust(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_trusts", scope, id)
}

func (r *Repo) AddEstateTrustDesignation(ctx context.Context, scope Scope, trustID, beneficiaryID uuid.UUID, share decimal.Decimal, tier string) (uuid.UUID, error) {
	return r.insertChildDesignation(ctx, scope, "estate_trust_designations", "trust_id", "estate_trusts", trustID, beneficiaryID, share, tier)
}

func (r *Repo) DeleteEstateTrustDesignation(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_trust_designations", scope, id)
}

func (r *Repo) EstateAssets(ctx context.Context, scope Scope) ([]EstateAsset, error) {
	rows, err := r.db.Query(ctx, `SELECT a.id,a.name,a.value,a.liquid,a.transfer_method,COALESCE(a.trust_id,'00000000-0000-0000-0000-000000000000'::uuid),COALESCE(t.name,'') FROM estate_assets a LEFT JOIN estate_trusts t ON t.id=a.trust_id AND t.user_id=a.user_id AND t.household_id=a.household_id WHERE a.user_id=$1 AND a.household_id=$2 ORDER BY a.created_at,a.id`, scope.UserID, scope.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateAsset{}
	for rows.Next() {
		var item EstateAsset
		if err := rows.Scan(&item.ID, &item.Name, &item.Value, &item.Liquid, &item.TransferMethod, &item.TrustID, &item.TrustName); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Owners, err = r.estateAssetOwners(ctx, scope, out[i].ID)
		if err != nil {
			return nil, err
		}
		out[i].Designations, err = r.estateDesignations(ctx, scope, "estate_asset_designations", "asset_id", out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Repo) CreateEstateAsset(ctx context.Context, scope Scope, item EstateAsset) (uuid.UUID, error) {
	var trustID any
	if item.TransferMethod == estate.TransferTrust && item.TrustID != uuid.Nil {
		trustID = item.TrustID
	}
	return r.insertScoped(ctx, scope, `INSERT INTO estate_assets(user_id,household_id,name,value,liquid,transfer_method,trust_id) SELECT h.user_id,h.id,$3,$4,$5,$6,t.id FROM households h LEFT JOIN estate_trusts t ON t.id=$7 AND t.user_id=h.user_id AND t.household_id=h.id WHERE h.id=$2 AND h.user_id=$1 AND h.archived_at IS NULL AND ($6<>'trust' OR t.id IS NOT NULL) RETURNING estate_assets.id`, strings.TrimSpace(item.Name), item.Value, item.Liquid, item.TransferMethod, trustID)
}

func (r *Repo) UpdateEstateAsset(ctx context.Context, scope Scope, item EstateAsset) error {
	var trustID any
	if item.TransferMethod == estate.TransferTrust && item.TrustID != uuid.Nil {
		trustID = item.TrustID
	}
	return r.updateScoped(ctx, `UPDATE estate_assets a SET name=$4,value=$5,liquid=$6,transfer_method=$7,trust_id=t.id,updated_at=now() FROM (SELECT id FROM estate_trusts WHERE id=$8 AND user_id=$2 AND household_id=$3 UNION ALL SELECT NULL::uuid WHERE $7<>'trust' LIMIT 1) t WHERE a.id=$1 AND a.user_id=$2 AND a.household_id=$3`, item.ID, scope, strings.TrimSpace(item.Name), item.Value, item.Liquid, item.TransferMethod, trustID)
}

func (r *Repo) DeleteEstateAsset(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_assets", scope, id)
}

func (r *Repo) AddEstateAssetOwner(ctx context.Context, scope Scope, assetID uuid.UUID, name string) (uuid.UUID, error) {
	return r.insertScoped(ctx, scope, `INSERT INTO estate_asset_owners(user_id,household_id,asset_id,owner_name) SELECT user_id,household_id,id,$4 FROM estate_assets WHERE id=$3 AND user_id=$1 AND household_id=$2 RETURNING estate_asset_owners.id`, assetID, strings.TrimSpace(name))
}

func (r *Repo) DeleteEstateAssetOwner(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_asset_owners", scope, id)
}

func (r *Repo) AddEstateAssetDesignation(ctx context.Context, scope Scope, assetID, beneficiaryID uuid.UUID, share decimal.Decimal, tier string) (uuid.UUID, error) {
	return r.insertChildDesignation(ctx, scope, "estate_asset_designations", "asset_id", "estate_assets", assetID, beneficiaryID, share, tier)
}

func (r *Repo) DeleteEstateAssetDesignation(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_asset_designations", scope, id)
}

func (r *Repo) EstateInsurancePolicies(ctx context.Context, scope Scope) ([]EstateInsurance, error) {
	rows, err := r.db.Query(ctx, `SELECT id,policy_name,benefit_amount,in_estate FROM estate_life_insurance WHERE user_id=$1 AND household_id=$2 ORDER BY created_at,id`, scope.UserID, scope.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateInsurance{}
	for rows.Next() {
		var item EstateInsurance
		if err := rows.Scan(&item.ID, &item.PolicyName, &item.Benefit, &item.InEstate); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Designations, err = r.estateDesignations(ctx, scope, "estate_insurance_designations", "insurance_id", out[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (r *Repo) CreateEstateInsurance(ctx context.Context, scope Scope, item EstateInsurance) (uuid.UUID, error) {
	return r.insertScoped(ctx, scope, `INSERT INTO estate_life_insurance(user_id,household_id,policy_name,benefit_amount,in_estate) SELECT user_id,id,$3,$4,$5 FROM households WHERE id=$2 AND user_id=$1 AND archived_at IS NULL RETURNING estate_life_insurance.id`, strings.TrimSpace(item.PolicyName), item.Benefit, item.InEstate)
}

func (r *Repo) UpdateEstateInsurance(ctx context.Context, scope Scope, item EstateInsurance) error {
	return r.updateScoped(ctx, `UPDATE estate_life_insurance SET policy_name=$4,benefit_amount=$5,in_estate=$6,updated_at=now() WHERE id=$1 AND user_id=$2 AND household_id=$3`, item.ID, scope, strings.TrimSpace(item.PolicyName), item.Benefit, item.InEstate)
}

func (r *Repo) DeleteEstateInsurance(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_life_insurance", scope, id)
}

func (r *Repo) AddEstateInsuranceDesignation(ctx context.Context, scope Scope, policyID, beneficiaryID uuid.UUID, share decimal.Decimal, tier string) (uuid.UUID, error) {
	return r.insertChildDesignation(ctx, scope, "estate_insurance_designations", "insurance_id", "estate_life_insurance", policyID, beneficiaryID, share, tier)
}

func (r *Repo) DeleteEstateInsuranceDesignation(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_insurance_designations", scope, id)
}

func (r *Repo) EstateDocuments(ctx context.Context, scope Scope) ([]EstateDocument, error) {
	rows, err := r.db.Query(ctx, `SELECT id,document_type,storage_location,status,notes FROM estate_legal_documents WHERE user_id=$1 AND household_id=$2 ORDER BY created_at,id`, scope.UserID, scope.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateDocument{}
	for rows.Next() {
		var item EstateDocument
		if err := rows.Scan(&item.ID, &item.DocumentType, &item.StorageLocation, &item.Status, &item.Notes); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repo) CreateEstateDocument(ctx context.Context, scope Scope, item EstateDocument) (uuid.UUID, error) {
	return r.insertScoped(ctx, scope, `INSERT INTO estate_legal_documents(user_id,household_id,document_type,storage_location,status,notes) SELECT user_id,id,$3,$4,$5,$6 FROM households WHERE id=$2 AND user_id=$1 AND archived_at IS NULL RETURNING estate_legal_documents.id`, strings.TrimSpace(item.DocumentType), strings.TrimSpace(item.StorageLocation), item.Status, strings.TrimSpace(item.Notes))
}
func (r *Repo) UpdateEstateDocument(ctx context.Context, scope Scope, item EstateDocument) error {
	return r.updateScoped(ctx, `UPDATE estate_legal_documents SET document_type=$4,storage_location=$5,status=$6,notes=$7,updated_at=now() WHERE id=$1 AND user_id=$2 AND household_id=$3`, item.ID, scope, strings.TrimSpace(item.DocumentType), strings.TrimSpace(item.StorageLocation), item.Status, strings.TrimSpace(item.Notes))
}
func (r *Repo) DeleteEstateDocument(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_legal_documents", scope, id)
}

func (r *Repo) EstateContacts(ctx context.Context, scope Scope) ([]EstateContact, error) {
	rows, err := r.db.Query(ctx, `SELECT id,role,name,contact_info,notes FROM estate_contacts WHERE user_id=$1 AND household_id=$2 ORDER BY created_at,id`, scope.UserID, scope.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateContact{}
	for rows.Next() {
		var item EstateContact
		if err := rows.Scan(&item.ID, &item.Role, &item.Name, &item.ContactInfo, &item.Notes); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (r *Repo) CreateEstateContact(ctx context.Context, scope Scope, item EstateContact) (uuid.UUID, error) {
	return r.insertScoped(ctx, scope, `INSERT INTO estate_contacts(user_id,household_id,role,name,contact_info,notes) SELECT user_id,id,$3,$4,$5,$6 FROM households WHERE id=$2 AND user_id=$1 AND archived_at IS NULL RETURNING estate_contacts.id`, strings.TrimSpace(item.Role), strings.TrimSpace(item.Name), strings.TrimSpace(item.ContactInfo), strings.TrimSpace(item.Notes))
}
func (r *Repo) UpdateEstateContact(ctx context.Context, scope Scope, item EstateContact) error {
	return r.updateScoped(ctx, `UPDATE estate_contacts SET role=$4,name=$5,contact_info=$6,notes=$7,updated_at=now() WHERE id=$1 AND user_id=$2 AND household_id=$3`, item.ID, scope, strings.TrimSpace(item.Role), strings.TrimSpace(item.Name), strings.TrimSpace(item.ContactInfo), strings.TrimSpace(item.Notes))
}
func (r *Repo) DeleteEstateContact(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_contacts", scope, id)
}

func (r *Repo) EstateDigitalAssets(ctx context.Context, scope Scope) ([]EstateDigitalAsset, error) {
	rows, err := r.db.Query(ctx, `SELECT id,service_name,account_identifier,access_instructions_location,notes FROM estate_digital_assets WHERE user_id=$1 AND household_id=$2 ORDER BY created_at,id`, scope.UserID, scope.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateDigitalAsset{}
	for rows.Next() {
		var item EstateDigitalAsset
		if err := rows.Scan(&item.ID, &item.ServiceName, &item.AccountIdentifier, &item.AccessInstructionsLocation, &item.Notes); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (r *Repo) CreateEstateDigitalAsset(ctx context.Context, scope Scope, item EstateDigitalAsset) (uuid.UUID, error) {
	if err := validateEstateDigitalMetadata(item); err != nil {
		return uuid.Nil, err
	}
	return r.insertScoped(ctx, scope, `INSERT INTO estate_digital_assets(user_id,household_id,service_name,account_identifier,access_instructions_location,notes) SELECT user_id,id,$3,$4,$5,$6 FROM households WHERE id=$2 AND user_id=$1 AND archived_at IS NULL RETURNING estate_digital_assets.id`, strings.TrimSpace(item.ServiceName), strings.TrimSpace(item.AccountIdentifier), strings.TrimSpace(item.AccessInstructionsLocation), strings.TrimSpace(item.Notes))
}
func (r *Repo) UpdateEstateDigitalAsset(ctx context.Context, scope Scope, item EstateDigitalAsset) error {
	if err := validateEstateDigitalMetadata(item); err != nil {
		return err
	}
	return r.updateScoped(ctx, `UPDATE estate_digital_assets SET service_name=$4,account_identifier=$5,access_instructions_location=$6,notes=$7,updated_at=now() WHERE id=$1 AND user_id=$2 AND household_id=$3`, item.ID, scope, strings.TrimSpace(item.ServiceName), strings.TrimSpace(item.AccountIdentifier), strings.TrimSpace(item.AccessInstructionsLocation), strings.TrimSpace(item.Notes))
}
func (r *Repo) DeleteEstateDigitalAsset(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_digital_assets", scope, id)
}

// ErrCredentialDetected rejects digital metadata that looks like it contains a
// credential. The message is deliberately generic: the offending value is never
// echoed, logged, or persisted.
var ErrCredentialDetected = errors.New("digital metadata must not contain credentials")

// validateEstateDigitalMetadata rejects credential-like values before insert or
// update. This is best-effort pattern matching, not cryptographic proof: it
// catches obvious credential labels and long high-entropy tokens while allowing
// ordinary metadata. The rejected value is never logged or persisted; callers
// surface only the generic ErrCredentialDetected reason.
func validateEstateDigitalMetadata(item EstateDigitalAsset) error {
	all := strings.ToLower(strings.Join([]string{item.ServiceName, item.AccountIdentifier, item.AccessInstructionsLocation, item.Notes}, "\n"))
	for _, label := range []string{"password:", "pwd:", "pin:", "seed phrase", "private key", "recovery code"} {
		if strings.Contains(all, label) {
			return ErrCredentialDetected
		}
	}
	for _, field := range []string{item.ServiceName, item.AccountIdentifier, item.AccessInstructionsLocation, item.Notes} {
		if hasHighEntropyToken(field) {
			return ErrCredentialDetected
		}
	}
	return nil
}

// hasHighEntropyToken reports whether s contains a contiguous run of 20+ letters
// and digits (no spaces) that looks like a generated secret: mixed case with
// digits, or a long hex-like token. Ordinary words and account numbers are too
// short or too uniform to match.
func hasHighEntropyToken(s string) bool {
	run := 0
	hasUpper, hasLower, hasDigit, hexOnly := false, false, false, true
	flush := func() bool {
		if run >= 20 && hasDigit && (hasUpper || hasLower) && (hasUpper && hasLower || hexOnly) {
			return true
		}
		run, hasUpper, hasLower, hasDigit, hexOnly = 0, false, false, false, true
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			run++
			hasUpper = true
			if r > 'F' {
				hexOnly = false
			}
		case r >= 'a' && r <= 'z':
			run++
			hasLower = true
			if r > 'f' {
				hexOnly = false
			}
		case r >= '0' && r <= '9':
			run++
			hasDigit = true
		default:
			if flush() {
				return true
			}
		}
	}
	return flush()
}

func (r *Repo) EstateResiduaryShares(ctx context.Context, scope Scope) ([]EstateResiduaryShare, error) {
	rows, err := r.db.Query(ctx, `SELECT r.id,r.beneficiary_id,b.name,r.share_percent FROM estate_residuary_shares r JOIN estate_beneficiaries b ON b.id=r.beneficiary_id AND b.user_id=r.user_id AND b.household_id=r.household_id WHERE r.user_id=$1 AND r.household_id=$2 ORDER BY b.name,r.id`, scope.UserID, scope.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateResiduaryShare{}
	for rows.Next() {
		var item EstateResiduaryShare
		if err := rows.Scan(&item.ID, &item.BeneficiaryID, &item.BeneficiaryName, &item.SharePercent); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (r *Repo) UpsertEstateResiduaryShare(ctx context.Context, scope Scope, beneficiaryID uuid.UUID, share decimal.Decimal) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `INSERT INTO estate_residuary_shares(user_id,household_id,beneficiary_id,share_percent) SELECT b.user_id,b.household_id,b.id,$4 FROM estate_beneficiaries b WHERE b.id=$3 AND b.user_id=$1 AND b.household_id=$2 ON CONFLICT (household_id,beneficiary_id) DO UPDATE SET share_percent=EXCLUDED.share_percent,updated_at=now() RETURNING estate_residuary_shares.id`, scope.UserID, scope.HouseholdID, beneficiaryID, share).Scan(&id)
	return id, estateInsertError(err)
}
func (r *Repo) DeleteEstateResiduaryShare(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_residuary_shares", scope, id)
}

func (r *Repo) EstateScenarios(ctx context.Context, scope Scope) ([]EstateScenario, error) {
	rows, err := r.db.Query(ctx, `SELECT id,name,decedent_name,as_of_year,death_year,debt_amount,funeral_cost,legal_cost,pay_debts FROM estate_scenarios WHERE user_id=$1 AND household_id=$2 ORDER BY created_at,id`, scope.UserID, scope.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateScenario{}
	for rows.Next() {
		var item EstateScenario
		if err := rows.Scan(&item.ID, &item.Name, &item.DecedentName, &item.AsOfYear, &item.DeathYear, &item.Debt, &item.FuneralCost, &item.LegalCost, &item.PayDebts); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
func (r *Repo) CreateEstateScenario(ctx context.Context, scope Scope, item EstateScenario) (uuid.UUID, error) {
	return r.insertScoped(ctx, scope, `INSERT INTO estate_scenarios(user_id,household_id,name,decedent_name,as_of_year,death_year,debt_amount,funeral_cost,legal_cost,pay_debts) SELECT user_id,id,$3,$4,$5,$6,$7,$8,$9,$10 FROM households WHERE id=$2 AND user_id=$1 AND archived_at IS NULL RETURNING estate_scenarios.id`, strings.TrimSpace(item.Name), strings.TrimSpace(item.DecedentName), item.AsOfYear, item.DeathYear, item.Debt, item.FuneralCost, item.LegalCost, item.PayDebts)
}
func (r *Repo) UpdateEstateScenario(ctx context.Context, scope Scope, item EstateScenario) error {
	return r.updateScoped(ctx, `UPDATE estate_scenarios SET name=$4,decedent_name=$5,as_of_year=$6,death_year=$7,debt_amount=$8,funeral_cost=$9,legal_cost=$10,pay_debts=$11,updated_at=now() WHERE id=$1 AND user_id=$2 AND household_id=$3`, item.ID, scope, strings.TrimSpace(item.Name), strings.TrimSpace(item.DecedentName), item.AsOfYear, item.DeathYear, item.Debt, item.FuneralCost, item.LegalCost, item.PayDebts)
}
func (r *Repo) DeleteEstateScenario(ctx context.Context, scope Scope, id uuid.UUID) error {
	return r.deleteScoped(ctx, "estate_scenarios", scope, id)
}

func (r *Repo) LoadEstateInput(ctx context.Context, scope Scope, scenarioID uuid.UUID) (estate.Input, error) {
	data, err := r.EstateData(ctx, scope)
	if err != nil {
		return estate.Input{}, err
	}
	var scenario EstateScenario
	for _, item := range data.Scenarios {
		if item.ID == scenarioID {
			scenario = item
			break
		}
	}
	if scenario.ID == uuid.Nil {
		return estate.Input{}, ErrNotFound
	}
	return estateInputFromData(data, scenario), nil
}

func estateInputFromData(data EstateData, scenario EstateScenario) estate.Input {
	in := estate.Input{AsOfYear: scenario.AsOfYear, DeathYear: scenario.DeathYear, DecedentName: scenario.DecedentName, Debt: scenario.Debt, FuneralCost: scenario.FuneralCost, LegalCost: scenario.LegalCost, PayDebts: scenario.PayDebts}
	for _, item := range data.Beneficiaries {
		in.Beneficiaries = append(in.Beneficiaries, estate.Beneficiary{ID: item.ID, Name: item.Name, Relationship: item.Relationship, Dependent: item.Dependent})
	}
	for _, item := range data.Trusts {
		trust := estate.Trust{ID: item.ID, Name: item.Name}
		for _, d := range item.Designations {
			trust.Designations = append(trust.Designations, estate.Designation{BeneficiaryID: d.BeneficiaryID, SharePercent: d.SharePercent, Tier: d.Tier})
		}
		in.Trusts = append(in.Trusts, trust)
	}
	for _, item := range data.Assets {
		asset := estate.Asset{ID: item.ID, Name: item.Name, Value: item.Value, Liquid: item.Liquid, TransferMethod: item.TransferMethod, TrustID: item.TrustID}
		for _, o := range item.Owners {
			asset.Owners = append(asset.Owners, estate.Owner{Name: o.Name})
		}
		for _, d := range item.Designations {
			asset.Designations = append(asset.Designations, estate.Designation{BeneficiaryID: d.BeneficiaryID, SharePercent: d.SharePercent, Tier: d.Tier})
		}
		in.Assets = append(in.Assets, asset)
	}
	for _, item := range data.Insurance {
		policy := estate.Insurance{ID: item.ID, PolicyName: item.PolicyName, Benefit: item.Benefit, InEstate: item.InEstate}
		for _, d := range item.Designations {
			policy.Designations = append(policy.Designations, estate.Designation{BeneficiaryID: d.BeneficiaryID, SharePercent: d.SharePercent, Tier: d.Tier})
		}
		in.Insurance = append(in.Insurance, policy)
	}
	for _, item := range data.Residuary {
		in.Residuary = append(in.Residuary, estate.ResiduaryShare{BeneficiaryID: item.BeneficiaryID, SharePercent: item.SharePercent})
	}
	return in
}

func estateReadinessInput(data EstateData, scenario EstateScenario) estate.ReadinessInput {
	documents := make([]estate.ReadinessDocument, 0, len(data.Documents))
	for _, item := range data.Documents {
		documents = append(documents, estate.ReadinessDocument{DocumentType: item.DocumentType, Status: item.Status})
	}
	contacts := make([]estate.ReadinessContact, 0, len(data.Contacts))
	for _, item := range data.Contacts {
		contacts = append(contacts, estate.ReadinessContact{Role: item.Role})
	}
	return estate.ReadinessInput{Plan: estateInputFromData(data, scenario), Documents: documents, Contacts: contacts, HasScenario: scenario.ID != uuid.Nil, ScenarioName: scenario.Name}
}

func (r *Repo) EstateReadiness(ctx context.Context, scope Scope, scenarioID uuid.UUID) ([]estate.ChecklistItem, error) {
	data, err := r.EstateData(ctx, scope)
	if err != nil {
		return nil, err
	}
	for _, scenario := range data.Scenarios {
		if scenario.ID == scenarioID {
			return estate.EvaluateReadiness(estateReadinessInput(data, scenario)), nil
		}
	}
	return nil, ErrNotFound
}

func (r *Repo) SaveEstateResult(ctx context.Context, scope Scope, scenarioID uuid.UUID, input estate.Input, result estate.Result) (uuid.UUID, error) {
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return uuid.Nil, err
	}
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = r.db.QueryRow(ctx, `INSERT INTO estate_results(user_id,household_id,scenario_id,scenario_name,decedent_name,as_of_year,death_year,input_snapshot,result_payload) SELECT user_id,household_id,id,name,decedent_name,as_of_year,death_year,$4,$5 FROM estate_scenarios WHERE id=$3 AND user_id=$1 AND household_id=$2 RETURNING estate_results.id`, scope.UserID, scope.HouseholdID, scenarioID, inputJSON, resultJSON).Scan(&id)
	return id, estateInsertError(err)
}

func (r *Repo) EstateResult(ctx context.Context, scope Scope, id uuid.UUID) (EstateResult, error) {
	var item EstateResult
	var inputJSON, resultJSON []byte
	err := r.db.QueryRow(ctx, `SELECT r.id,r.scenario_id,r.input_snapshot,r.result_payload,r.created_at,r.scenario_name,r.decedent_name,r.as_of_year,r.death_year FROM estate_results r WHERE r.id=$1 AND r.user_id=$2 AND r.household_id=$3`, id, scope.UserID, scope.HouseholdID).Scan(&item.ID, &item.ScenarioID, &inputJSON, &resultJSON, &item.CreatedAt, &item.ScenarioName, &item.DecedentName, &item.AsOfYear, &item.DeathYear)
	if errors.Is(err, pgx.ErrNoRows) {
		return EstateResult{}, ErrNotFound
	}
	if err != nil {
		return EstateResult{}, err
	}
	if err := json.Unmarshal(inputJSON, &item.Input); err != nil {
		return EstateResult{}, err
	}
	if err := json.Unmarshal(resultJSON, &item.Result); err != nil {
		return EstateResult{}, err
	}
	return item, nil
}

func (r *Repo) EstateResults(ctx context.Context, scope Scope) ([]EstateResult, error) {
	rows, err := r.db.Query(ctx, `SELECT r.id,r.scenario_id,r.created_at,r.scenario_name,r.decedent_name,r.as_of_year,r.death_year FROM estate_results r WHERE r.user_id=$1 AND r.household_id=$2 ORDER BY r.created_at DESC,r.id`, scope.UserID, scope.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateResult{}
	for rows.Next() {
		var item EstateResult
		if err := rows.Scan(&item.ID, &item.ScenarioID, &item.CreatedAt, &item.ScenarioName, &item.DecedentName, &item.AsOfYear, &item.DeathYear); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repo) estateAssetOwners(ctx context.Context, scope Scope, assetID uuid.UUID) ([]EstateOwner, error) {
	rows, err := r.db.Query(ctx, `SELECT id,asset_id,owner_name FROM estate_asset_owners WHERE user_id=$1 AND household_id=$2 AND asset_id=$3 ORDER BY owner_name,id`, scope.UserID, scope.HouseholdID, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateOwner{}
	for rows.Next() {
		var item EstateOwner
		if err := rows.Scan(&item.ID, &item.AssetID, &item.Name); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repo) estateDesignations(ctx context.Context, scope Scope, table, parentColumn string, parentID uuid.UUID) ([]EstateDesignation, error) {
	allowed := map[string]string{"estate_trust_designations": "trust_id", "estate_asset_designations": "asset_id", "estate_insurance_designations": "insurance_id"}
	if allowed[table] != parentColumn {
		return nil, ErrNotFound
	}
	query := fmt.Sprintf(`SELECT d.id,d.%s,d.beneficiary_id,b.name,d.share_percent,d.tier FROM %s d JOIN estate_beneficiaries b ON b.id=d.beneficiary_id AND b.user_id=d.user_id AND b.household_id=d.household_id WHERE d.user_id=$1 AND d.household_id=$2 AND d.%s=$3 ORDER BY d.tier,b.name,d.id`, parentColumn, table, parentColumn)
	rows, err := r.db.Query(ctx, query, scope.UserID, scope.HouseholdID, parentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EstateDesignation{}
	for rows.Next() {
		var item EstateDesignation
		if err := rows.Scan(&item.ID, &item.ParentID, &item.BeneficiaryID, &item.BeneficiaryName, &item.SharePercent, &item.Tier); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repo) insertChildDesignation(ctx context.Context, scope Scope, table, parentColumn, parentTable string, parentID, beneficiaryID uuid.UUID, share decimal.Decimal, tier string) (uuid.UUID, error) {
	allowed := map[string]string{"estate_trust_designations": "estate_trusts", "estate_asset_designations": "estate_assets", "estate_insurance_designations": "estate_life_insurance"}
	if allowed[table] != parentTable {
		return uuid.Nil, ErrNotFound
	}
	query := fmt.Sprintf(`INSERT INTO %s(user_id,household_id,%s,beneficiary_id,share_percent,tier) SELECT p.user_id,p.household_id,p.id,b.id,$5,$6 FROM %s p JOIN estate_beneficiaries b ON b.id=$4 AND b.user_id=p.user_id AND b.household_id=p.household_id WHERE p.id=$3 AND p.user_id=$1 AND p.household_id=$2 RETURNING %s.id`, table, parentColumn, parentTable, table)
	var id uuid.UUID
	err := r.db.QueryRow(ctx, query, scope.UserID, scope.HouseholdID, parentID, beneficiaryID, share, tier).Scan(&id)
	return id, estateInsertError(err)
}

func (r *Repo) insertScoped(ctx context.Context, scope Scope, query string, args ...any) (uuid.UUID, error) {
	params := []any{scope.UserID, scope.HouseholdID}
	params = append(params, args...)
	var id uuid.UUID
	err := r.db.QueryRow(ctx, query, params...).Scan(&id)
	return id, estateInsertError(err)
}

func (r *Repo) updateScoped(ctx context.Context, query string, id uuid.UUID, scope Scope, args ...any) error {
	params := []any{id, scope.UserID, scope.HouseholdID}
	params = append(params, args...)
	tag, err := r.db.Exec(ctx, query, params...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) deleteScoped(ctx context.Context, table string, scope Scope, id uuid.UUID) error {
	allowed := map[string]bool{"estate_beneficiaries": true, "estate_trusts": true, "estate_trust_designations": true, "estate_assets": true, "estate_asset_owners": true, "estate_asset_designations": true, "estate_life_insurance": true, "estate_insurance_designations": true, "estate_legal_documents": true, "estate_contacts": true, "estate_digital_assets": true, "estate_residuary_shares": true, "estate_scenarios": true}
	if !allowed[table] {
		return ErrNotFound
	}
	tag, err := r.db.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE id=$1 AND user_id=$2 AND household_id=$3", table), id, scope.UserID, scope.HouseholdID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func estateInsertError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
