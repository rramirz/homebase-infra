//go:build integration

package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/internal/app"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/pkg/estate"
	"github.com/shopspring/decimal"
)

func TestEstateRoutesRequireOwnedHouseholdScope(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL or DATABASE_URL required")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repo := app.NewRepo(db)
	if err := repo.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	u1, err := repo.Register(ctx, uuid.NewString()+"@routes.test", "password1234")
	if err != nil {
		t.Fatal(err)
	}
	u2, err := repo.Register(ctx, uuid.NewString()+"@routes.test", "password1234")
	if err != nil {
		t.Fatal(err)
	}
	h1, err := repo.CreateHousehold(ctx, u1, "Owner")
	if err != nil {
		t.Fatal(err)
	}
	h2, err := repo.CreateHousehold(ctx, u2, "Other")
	if err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	private := router.Group("", func(c *gin.Context) { c.Set("user", u1) })
	templates := app.Templates()
	render := func(c *gin.Context, name string, data page) {
		data["CSRF"] = "test"
		if h, ok := c.Get("household"); ok {
			data["Household"] = h
		}
		if err := templates.ExecuteTemplate(c.Writer, name, data); err != nil {
			t.Errorf("render %s: %v", name, err)
		}
	}
	registerEstateRoutes(private, repo, render)

	request := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}
	if got := request(http.MethodGet, "/households/"+h1.String()+"/estate/", ""); got.Code != http.StatusOK || !strings.Contains(got.Body.String(), "Estate planning workspace") {
		t.Fatalf("owned route: %d %s", got.Code, got.Body.String())
	}
	if got := request(http.MethodGet, "/households/"+h2.String()+"/estate/", ""); got.Code != http.StatusNotFound {
		t.Fatalf("cross-household route = %d", got.Code)
	}
	createPath := "/households/" + h1.String() + "/estate/beneficiaries"
	if got := request(http.MethodPost, createPath, "name=Route+Beneficiary&relationship=child&dependent=true"); got.Code != http.StatusSeeOther {
		t.Fatalf("create route = %d %s", got.Code, got.Body.String())
	}
	items, err := repo.EstateBeneficiaries(ctx, app.Scope{UserID: u1, HouseholdID: h1})
	if err != nil || len(items) != 1 || items[0].Name != "Route Beneficiary" || !items[0].Dependent {
		t.Fatalf("route did not persist: %v %#v", err, items)
	}
	if got := request(http.MethodPost, "/households/"+h2.String()+"/estate/beneficiaries", "name=Attack"); got.Code != http.StatusNotFound {
		t.Fatalf("cross-household write route = %d", got.Code)
	}
	scope := app.Scope{UserID: u1, HouseholdID: h1}
	assetID, err := repo.CreateEstateAsset(ctx, scope, app.EstateAsset{Name: "Route TOD", Value: decimal.NewFromInt(100), TransferMethod: estate.TransferDesignate})
	if err != nil {
		t.Fatal(err)
	}
	designationPath := "/households/" + h1.String() + "/estate/assets/" + assetID.String() + "/designations"
	designationBody := "beneficiary_id=" + items[0].ID.String() + "&share_percent=1&tier=contingent"
	if got := request(http.MethodPost, designationPath, designationBody); got.Code != http.StatusSeeOther {
		t.Fatalf("designation route = %d %s", got.Code, got.Body.String())
	}
	assets, err := repo.EstateAssets(ctx, scope)
	if err != nil || len(assets) != 1 || len(assets[0].Designations) != 1 || assets[0].Designations[0].Tier != estate.TierContingent {
		t.Fatalf("tier route did not persist: %v %#v", err, assets)
	}

	scenarioPath := "/households/" + h1.String() + "/estate/scenarios"
	scenarioBody := "name=Route+scenario&decedent_name=Morgan&as_of_year=2026&death_year=2032&debt=100&funeral_cost=20&legal_cost=30&pay_debts=true"
	if got := request(http.MethodPost, scenarioPath, scenarioBody); got.Code != http.StatusSeeOther {
		t.Fatalf("scenario route = %d %s", got.Code, got.Body.String())
	}
	scenarios, err := repo.EstateScenarios(ctx, scope)
	if err != nil || len(scenarios) != 1 || scenarios[0].DeathYear != 2032 || !scenarios[0].FuneralCost.Equal(decimal.NewFromInt(20)) || !scenarios[0].LegalCost.Equal(decimal.NewFromInt(30)) || !scenarios[0].PayDebts {
		t.Fatalf("scenario fields did not persist: %v %#v", err, scenarios)
	}
}
