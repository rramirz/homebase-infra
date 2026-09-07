package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

var ErrNotFound = errors.New("not found")

type Scope struct{ UserID, HouseholdID uuid.UUID }
type Household struct {
	ID       uuid.UUID
	Name     string
	Archived bool
}
type Resource struct {
	ID                                                                   uuid.UUID
	Name, Kind, Amount, StartYear, EndYear, Inflation, Extra, Confidence string
}
type Plan struct {
	ID                                  uuid.UUID
	Name                                string
	AsOfYear, EndYear                   int
	AnnualReturn, Inflation, Confidence string
}
type Scenario struct {
	ID        uuid.UUID
	Name      string
	Overrides []byte
}
type Repo struct{ db *pgxpool.Pool }

func NewRepo(db *pgxpool.Pool) *Repo { return &Repo{db: db} }
func (r *Repo) Migrate(ctx context.Context) error {
	if _, err := r.db.Exec(ctx, "CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())"); err != nil {
		return err
	}
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		var applied bool
		if err := r.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)", e.Name()).Scan(&applied); err != nil {
			return err
		}
		if applied {
			continue
		}
		b, err := migrationFiles.ReadFile("migrations/" + e.Name())
		if err != nil {
			return err
		}
		tx, err := r.db.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, string(b)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO schema_migrations(version) VALUES($1)", e.Name()); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err = tx.Commit(ctx); err != nil {
			return fmt.Errorf("%s: %w", e.Name(), err)
		}
	}
	return nil
}
func hashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", errors.New("password must be at least 12 characters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
	return "argon2id$" + hex.EncodeToString(salt) + "$" + hex.EncodeToString(h), nil
}
func verifyPassword(encoded, password string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 3 || p[0] != "argon2id" {
		return false
	}
	salt, e1 := hex.DecodeString(p[1])
	expected, e2 := hex.DecodeString(p[2])
	if e1 != nil || e2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, uint32(len(expected)))
	return subtleEqual(got, expected)
}
func subtleEqual(a, b []byte) bool { return subtle.ConstantTimeCompare(a, b) == 1 }
func (r *Repo) Register(ctx context.Context, email, password string) (uuid.UUID, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return uuid.Nil, errors.New("invalid email")
	}
	h, err := hashPassword(password)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = r.db.QueryRow(ctx, "INSERT INTO users(email,password_hash) VALUES(lower($1),$2) RETURNING id", email, h).Scan(&id)
	return id, err
}
func (r *Repo) Login(ctx context.Context, email, password string) (uuid.UUID, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return uuid.Nil, "", ErrNotFound
	}
	var id uuid.UUID
	var h string
	err := r.db.QueryRow(ctx, "SELECT id,password_hash FROM users WHERE email=lower($1)", email).Scan(&id, &h)
	if err != nil || !verifyPassword(h, password) {
		return uuid.Nil, "", ErrNotFound
	}
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return uuid.Nil, "", err
	}
	token := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, "DELETE FROM sessions WHERE user_id=$1", id); err != nil {
		return uuid.Nil, "", err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO sessions(user_id,token_hash,expires_at) VALUES($1,$2,$3)", id, sum[:], time.Now().Add(14*24*time.Hour)); err != nil {
		return uuid.Nil, "", err
	}
	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, "", err
	}
	return id, token, err
}
func (r *Repo) UserForSession(ctx context.Context, token string) (uuid.UUID, error) {
	sum := sha256.Sum256([]byte(token))
	var id uuid.UUID
	err := r.db.QueryRow(ctx, "SELECT user_id FROM sessions WHERE token_hash=$1 AND expires_at>now()", sum[:]).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}
func (r *Repo) Logout(ctx context.Context, token string) error {
	sum := sha256.Sum256([]byte(token))
	_, err := r.db.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", sum[:])
	return err
}
func (r *Repo) Households(ctx context.Context, user uuid.UUID) ([]Household, error) {
	rows, err := r.db.Query(ctx, "SELECT id,name,archived_at IS NOT NULL FROM households WHERE user_id=$1 ORDER BY created_at", user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Household{}
	for rows.Next() {
		var h Household
		if err := rows.Scan(&h.ID, &h.Name, &h.Archived); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func (r *Repo) CreateHousehold(ctx context.Context, user uuid.UUID, name string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, "INSERT INTO households(user_id,name) VALUES($1,$2) RETURNING id", user, name).Scan(&id)
	return id, err
}
func (r *Repo) ArchiveHousehold(ctx context.Context, user, id uuid.UUID) error {
	tag, err := r.db.Exec(ctx, "UPDATE households SET archived_at=now() WHERE id=$1 AND user_id=$2", id, user)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

var resourceTables = map[string]struct{ columns string }{"people": {"name,birth_year,retirement_year,death_year"}, "accounts": {"name,kind,balance"}, "liabilities": {"name,balance,annual_rate,annual_payment"}, "incomes": {"name,kind,annual_amount,start_year,end_year,inflation_rate"}, "expenses": {"name,kind,annual_amount,start_year,end_year,inflation_rate"}}

func resourceTable(table string) (string, error) {
	if _, ok := resourceTables[table]; !ok {
		return "", ErrNotFound
	}
	return table, nil
}
func (r *Repo) ListResources(ctx context.Context, s Scope, table string) ([]Resource, error) {
	t, err := resourceTable(table)
	if err != nil {
		return nil, err
	}
	var q string
	switch t {
	case "people":
		q = "SELECT id,name,'',birth_year::text,COALESCE(retirement_year::text,''),COALESCE(death_year::text,''),'',confidence FROM people WHERE user_id=$1 AND household_id=$2 ORDER BY created_at"
	case "accounts":
		q = "SELECT id,name,kind,balance::text,'','','',confidence FROM accounts WHERE user_id=$1 AND household_id=$2 ORDER BY created_at"
	case "liabilities":
		q = "SELECT id,name,'',balance::text,'',annual_payment::text,annual_rate::text,confidence FROM liabilities WHERE user_id=$1 AND household_id=$2 ORDER BY created_at"
	case "incomes":
		q = "SELECT id,name,kind,annual_amount::text,start_year::text,COALESCE(end_year::text,''),inflation_rate::text,confidence FROM incomes WHERE user_id=$1 AND household_id=$2 ORDER BY created_at"
	case "expenses":
		q = "SELECT id,name,kind,annual_amount::text,start_year::text,COALESCE(end_year::text,''),inflation_rate::text,confidence FROM expenses WHERE user_id=$1 AND household_id=$2 ORDER BY created_at"
	}
	rows, err := r.db.Query(ctx, q, s.UserID, s.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Resource
	for rows.Next() {
		var x Resource
		if err = rows.Scan(&x.ID, &x.Name, &x.Kind, &x.Amount, &x.StartYear, &x.EndYear, &x.Inflation, &x.Confidence); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *Repo) CreateResource(ctx context.Context, s Scope, table string, x Resource) error {
	t, err := resourceTable(table)
	if err != nil {
		return err
	}
	var q string
	switch t {
	case "people":
		q = "INSERT INTO people(user_id,household_id,name,birth_year,retirement_year,death_year,confidence) VALUES($1,$2,$3,$4::int,NULLIF($5,'')::int,NULLIF($6,'')::int,COALESCE(NULLIF($7,''),'unknown'))"
		_, err = r.db.Exec(ctx, q, s.UserID, s.HouseholdID, x.Name, x.Amount, x.StartYear, x.EndYear, x.Confidence)
	case "accounts":
		q = "INSERT INTO accounts(user_id,household_id,name,kind,balance,confidence) VALUES($1,$2,$3,$4,$5::numeric,COALESCE(NULLIF($6,''),'unknown'))"
		_, err = r.db.Exec(ctx, q, s.UserID, s.HouseholdID, x.Name, x.Kind, x.Amount, x.Confidence)
	case "liabilities":
		q = "INSERT INTO liabilities(user_id,household_id,name,balance,annual_rate,annual_payment,confidence) VALUES($1,$2,$3,$4::numeric,$5::numeric,$6::numeric,COALESCE(NULLIF($7,''),'unknown'))"
		_, err = r.db.Exec(ctx, q, s.UserID, s.HouseholdID, x.Name, x.Amount, x.Inflation, x.EndYear, x.Confidence)
	default:
		q = fmt.Sprintf("INSERT INTO %s(user_id,household_id,name,kind,annual_amount,start_year,end_year,inflation_rate,confidence) VALUES($1,$2,$3,$4,$5::numeric,$6::int,NULLIF($7,'')::int,$8::numeric,COALESCE(NULLIF($9,''),'unknown'))", t)
		_, err = r.db.Exec(ctx, q, s.UserID, s.HouseholdID, x.Name, x.Kind, x.Amount, x.StartYear, x.EndYear, x.Inflation, x.Confidence)
	}
	return err
}
func (r *Repo) DeleteResource(ctx context.Context, s Scope, table string, id uuid.UUID) error {
	t, err := resourceTable(table)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, fmt.Sprintf("DELETE FROM %s WHERE id=$1 AND user_id=$2 AND household_id=$3", t), id, s.UserID, s.HouseholdID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateResource preserves scope: a mismatched user or household is indistinguishable from not found.
func (r *Repo) UpdateResource(ctx context.Context, s Scope, table string, x Resource) error {
	t, err := resourceTable(table)
	if err != nil {
		return err
	}
	var q string
	var args []any
	switch t {
	case "people":
		q = "UPDATE people SET name=$4,birth_year=$5::int,retirement_year=NULLIF($6,'')::int,death_year=NULLIF($7,'')::int,confidence=COALESCE(NULLIF($8,''),'unknown') WHERE id=$1 AND user_id=$2 AND household_id=$3"
		args = []any{x.ID, s.UserID, s.HouseholdID, x.Name, x.Amount, x.StartYear, x.EndYear, x.Confidence}
	case "accounts":
		q = "UPDATE accounts SET name=$4,kind=$5,balance=$6::numeric,confidence=COALESCE(NULLIF($7,''),'unknown') WHERE id=$1 AND user_id=$2 AND household_id=$3"
		args = []any{x.ID, s.UserID, s.HouseholdID, x.Name, x.Kind, x.Amount, x.Confidence}
	case "liabilities":
		q = "UPDATE liabilities SET name=$4,balance=$5::numeric,annual_rate=$6::numeric,annual_payment=$7::numeric,confidence=COALESCE(NULLIF($8,''),'unknown') WHERE id=$1 AND user_id=$2 AND household_id=$3"
		args = []any{x.ID, s.UserID, s.HouseholdID, x.Name, x.Amount, x.Inflation, x.EndYear, x.Confidence}
	default:
		q = fmt.Sprintf("UPDATE %s SET name=$4,kind=$5,annual_amount=$6::numeric,start_year=$7::int,end_year=NULLIF($8,'')::int,inflation_rate=$9::numeric,confidence=COALESCE(NULLIF($10,''),'unknown') WHERE id=$1 AND user_id=$2 AND household_id=$3", t)
		args = []any{x.ID, s.UserID, s.HouseholdID, x.Name, x.Kind, x.Amount, x.StartYear, x.EndYear, x.Inflation, x.Confidence}
	}
	tag, err := r.db.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
func (r *Repo) Plans(ctx context.Context, s Scope) ([]Plan, error) {
	rows, err := r.db.Query(ctx, "SELECT id,name,as_of_year,end_year,annual_return::text,inflation_rate::text,confidence FROM retirement_plans WHERE user_id=$1 AND household_id=$2 ORDER BY created_at", s.UserID, s.HouseholdID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Plan
	for rows.Next() {
		var p Plan
		if err := rows.Scan(&p.ID, &p.Name, &p.AsOfYear, &p.EndYear, &p.AnnualReturn, &p.Inflation, &p.Confidence); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (r *Repo) CreatePlan(ctx context.Context, s Scope, p Plan) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, "INSERT INTO retirement_plans(user_id,household_id,name,as_of_year,end_year,annual_return,inflation_rate,confidence) VALUES($1,$2,$3,$4,$5,$6::numeric,$7::numeric,COALESCE(NULLIF($8,''),'unknown')) RETURNING id", s.UserID, s.HouseholdID, p.Name, p.AsOfYear, p.EndYear, p.AnnualReturn, p.Inflation, p.Confidence).Scan(&id)
	return id, err
}
func (r *Repo) UpdatePlan(ctx context.Context, s Scope, p Plan) error {
	tag, err := r.db.Exec(ctx, "UPDATE retirement_plans SET name=$4,as_of_year=$5,end_year=$6,annual_return=$7::numeric,inflation_rate=$8::numeric,confidence=COALESCE(NULLIF($9,''),'unknown') WHERE id=$1 AND user_id=$2 AND household_id=$3", p.ID, s.UserID, s.HouseholdID, p.Name, p.AsOfYear, p.EndYear, p.AnnualReturn, p.Inflation, p.Confidence)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
func (r *Repo) Scenarios(ctx context.Context, s Scope, planID uuid.UUID) ([]Scenario, error) {
	rows, err := r.db.Query(ctx, "SELECT id,name,overrides FROM scenarios WHERE user_id=$1 AND household_id=$2 AND plan_id=$3 ORDER BY created_at", s.UserID, s.HouseholdID, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Scenario
	for rows.Next() {
		var x Scenario
		if err := rows.Scan(&x.ID, &x.Name, &x.Overrides); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
func (r *Repo) CreateScenario(ctx context.Context, s Scope, planID uuid.UUID, name string, overrides []byte) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, "INSERT INTO scenarios(user_id,household_id,plan_id,name,overrides) SELECT $1,$2,id,$4,$5::jsonb FROM retirement_plans WHERE id=$3 AND user_id=$1 AND household_id=$2 RETURNING id", s.UserID, s.HouseholdID, planID, name, overrides).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	return id, err
}
func (r *Repo) CloneScenario(ctx context.Context, s Scope, sourceID uuid.UUID, name string) (uuid.UUID, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id uuid.UUID
	err = tx.QueryRow(ctx, "INSERT INTO scenarios(user_id,household_id,plan_id,name,overrides) SELECT user_id,household_id,plan_id,$4,overrides FROM scenarios WHERE id=$1 AND user_id=$2 AND household_id=$3 RETURNING id", sourceID, s.UserID, s.HouseholdID, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, ErrNotFound
	}
	if err != nil {
		return uuid.Nil, err
	}
	return id, tx.Commit(ctx)
}
func (r *Repo) UpdateScenario(ctx context.Context, s Scope, x Scenario) error {
	tag, err := r.db.Exec(ctx, "UPDATE scenarios SET name=$4,overrides=$5::jsonb WHERE id=$1 AND user_id=$2 AND household_id=$3", x.ID, s.UserID, s.HouseholdID, x.Name, x.Overrides)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) UpdateHousehold(ctx context.Context, user uuid.UUID, id uuid.UUID, name string) error {
	tag, err := r.db.Exec(ctx, "UPDATE households SET name=$1 WHERE id=$2 AND user_id=$3 AND archived_at IS NULL", name, id, user)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
