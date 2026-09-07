package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/internal/app"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/pkg/estate"
	"github.com/shopspring/decimal"
)

const estateDisclaimer = "Informational planning inventory and deterministic estimate only. Not legal, tax, probate, or financial advice. It does not create or validate a will, trust, beneficiary designation, title, or estate plan. Review all decisions and documents with qualified professionals."
const credentialWarning = "Store metadata and the location of access instructions only. Never enter passwords, PINs, recovery codes, private keys, security answers, or other credentials. Keep credentials in a password manager."

func registerEstateRoutes(private *gin.RouterGroup, repo *app.Repo, render func(c *gin.Context, name string, p page)) {
	routes := private.Group("/households/:id/estate")

	routes.GET("/", func(c *gin.Context) {
		scope, data, ok := loadEstateData(c, repo)
		if !ok {
			return
		}
		renderEstate(c, render, "estate_dashboard.html", scope, page{"Data": data})
	})

	routes.GET("/beneficiaries", func(c *gin.Context) {
		scope, data, ok := loadEstateData(c, repo)
		if !ok {
			return
		}
		renderEstate(c, render, "estate_beneficiaries.html", scope, page{"Items": data.Beneficiaries})
	})
	routes.POST("/beneficiaries", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		_, err := repo.CreateEstateBeneficiary(c, scope, app.EstateBeneficiary{Name: c.PostForm("name"), Relationship: c.PostForm("relationship"), Dependent: c.PostForm("dependent") == "true"})
		estateMutation(c, err, c.Request.URL.Path)
	})
	routes.POST("/beneficiaries/:item/update", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		err := repo.UpdateEstateBeneficiary(c, scope, app.EstateBeneficiary{ID: parseID(c.Param("item")), Name: c.PostForm("name"), Relationship: c.PostForm("relationship"), Dependent: c.PostForm("dependent") == "true"})
		estateMutation(c, err, estateURL(scope)+"/beneficiaries")
	})
	routes.POST("/beneficiaries/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateBeneficiary(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/beneficiaries")
	})

	routes.GET("/assets", func(c *gin.Context) {
		scope, data, ok := loadEstateData(c, repo)
		if !ok {
			return
		}
		renderEstate(c, render, "estate_assets.html", scope, page{"Assets": data.Assets, "Trusts": data.Trusts, "Beneficiaries": data.Beneficiaries})
	})
	routes.POST("/trusts", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		_, err := repo.CreateEstateTrust(c, scope, app.EstateTrust{Name: c.PostForm("name"), Notes: c.PostForm("notes")})
		estateMutation(c, err, estateURL(scope)+"/assets")
	})
	routes.POST("/trusts/:item/update", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		err := repo.UpdateEstateTrust(c, scope, app.EstateTrust{ID: parseID(c.Param("item")), Name: c.PostForm("name"), Notes: c.PostForm("notes")})
		estateMutation(c, err, estateURL(scope)+"/assets")
	})
	routes.POST("/trusts/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateTrust(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/assets")
	})
	routes.POST("/trusts/:item/designations", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		share, err := estateDecimal(c.PostForm("share_percent"))
		if err == nil {
			_, err = repo.AddEstateTrustDesignation(c, scope, parseID(c.Param("item")), parseID(c.PostForm("beneficiary_id")), share, c.PostForm("tier"))
		}
		estateMutation(c, err, estateURL(scope)+"/assets")
	})
	routes.POST("/trust-designations/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateTrustDesignation(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/assets")
	})
	routes.POST("/assets", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		value, err := estateDecimal(c.PostForm("value"))
		if err == nil {
			_, err = repo.CreateEstateAsset(c, scope, app.EstateAsset{Name: c.PostForm("name"), Value: value, Liquid: c.PostForm("liquid") == "true", TransferMethod: c.PostForm("transfer_method"), TrustID: parseID(c.PostForm("trust_id"))})
		}
		estateMutation(c, err, c.Request.URL.Path)
	})
	routes.POST("/assets/:item/update", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		value, err := estateDecimal(c.PostForm("value"))
		if err == nil {
			err = repo.UpdateEstateAsset(c, scope, app.EstateAsset{ID: parseID(c.Param("item")), Name: c.PostForm("name"), Value: value, Liquid: c.PostForm("liquid") == "true", TransferMethod: c.PostForm("transfer_method"), TrustID: parseID(c.PostForm("trust_id"))})
		}
		estateMutation(c, err, estateURL(scope)+"/assets")
	})
	routes.POST("/assets/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateAsset(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/assets")
	})
	routes.POST("/assets/:item/owners", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		_, err := repo.AddEstateAssetOwner(c, scope, parseID(c.Param("item")), c.PostForm("owner_name"))
		estateMutation(c, err, estateURL(scope)+"/assets")
	})
	routes.POST("/asset-owners/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateAssetOwner(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/assets")
	})
	routes.POST("/assets/:item/designations", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		share, err := estateDecimal(c.PostForm("share_percent"))
		if err == nil {
			_, err = repo.AddEstateAssetDesignation(c, scope, parseID(c.Param("item")), parseID(c.PostForm("beneficiary_id")), share, c.PostForm("tier"))
		}
		estateMutation(c, err, estateURL(scope)+"/assets")
	})
	routes.POST("/asset-designations/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateAssetDesignation(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/assets")
	})

	routes.GET("/insurance", func(c *gin.Context) {
		scope, data, ok := loadEstateData(c, repo)
		if !ok {
			return
		}
		renderEstate(c, render, "estate_insurance.html", scope, page{"Items": data.Insurance, "Beneficiaries": data.Beneficiaries})
	})
	routes.POST("/insurance", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		benefit, err := estateDecimal(c.PostForm("benefit"))
		if err == nil {
			_, err = repo.CreateEstateInsurance(c, scope, app.EstateInsurance{PolicyName: c.PostForm("policy_name"), Benefit: benefit, InEstate: c.PostForm("in_estate") == "true"})
		}
		estateMutation(c, err, c.Request.URL.Path)
	})
	routes.POST("/insurance/:item/update", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		benefit, err := estateDecimal(c.PostForm("benefit"))
		if err == nil {
			err = repo.UpdateEstateInsurance(c, scope, app.EstateInsurance{ID: parseID(c.Param("item")), PolicyName: c.PostForm("policy_name"), Benefit: benefit, InEstate: c.PostForm("in_estate") == "true"})
		}
		estateMutation(c, err, estateURL(scope)+"/insurance")
	})
	routes.POST("/insurance/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateInsurance(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/insurance")
	})
	routes.POST("/insurance/:item/designations", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		share, err := estateDecimal(c.PostForm("share_percent"))
		if err == nil {
			_, err = repo.AddEstateInsuranceDesignation(c, scope, parseID(c.Param("item")), parseID(c.PostForm("beneficiary_id")), share, c.PostForm("tier"))
		}
		estateMutation(c, err, estateURL(scope)+"/insurance")
	})
	routes.POST("/insurance-designations/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateInsuranceDesignation(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/insurance")
	})

	routes.GET("/documents", func(c *gin.Context) {
		scope, data, ok := loadEstateData(c, repo)
		if !ok {
			return
		}
		renderEstate(c, render, "estate_documents.html", scope, page{"Items": data.Documents})
	})
	routes.POST("/documents", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		_, err := repo.CreateEstateDocument(c, scope, estateDocumentForm(c, uuid.Nil))
		estateMutation(c, err, c.Request.URL.Path)
	})
	routes.POST("/documents/:item/update", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.UpdateEstateDocument(c, scope, estateDocumentForm(c, parseID(c.Param("item")))), estateURL(scope)+"/documents")
	})
	routes.POST("/documents/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateDocument(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/documents")
	})

	routes.GET("/contacts", func(c *gin.Context) {
		scope, data, ok := loadEstateData(c, repo)
		if !ok {
			return
		}
		renderEstate(c, render, "estate_contacts.html", scope, page{"Items": data.Contacts})
	})
	routes.POST("/contacts", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		_, err := repo.CreateEstateContact(c, scope, estateContactForm(c, uuid.Nil))
		estateMutation(c, err, c.Request.URL.Path)
	})
	routes.POST("/contacts/:item/update", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.UpdateEstateContact(c, scope, estateContactForm(c, parseID(c.Param("item")))), estateURL(scope)+"/contacts")
	})
	routes.POST("/contacts/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateContact(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/contacts")
	})

	routes.GET("/digital", func(c *gin.Context) {
		scope, data, ok := loadEstateData(c, repo)
		if !ok {
			return
		}
		renderEstate(c, render, "estate_digital.html", scope, page{"Items": data.DigitalAssets, "CredentialWarning": credentialWarning})
	})
	routes.POST("/digital", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		_, err := repo.CreateEstateDigitalAsset(c, scope, estateDigitalForm(c, uuid.Nil))
		estateMutation(c, err, c.Request.URL.Path)
	})
	routes.POST("/digital/:item/update", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.UpdateEstateDigitalAsset(c, scope, estateDigitalForm(c, parseID(c.Param("item")))), estateURL(scope)+"/digital")
	})
	routes.POST("/digital/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateDigitalAsset(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/digital")
	})

	routes.GET("/scenarios", func(c *gin.Context) {
		scope, data, ok := loadEstateData(c, repo)
		if !ok {
			return
		}
		renderEstate(c, render, "estate_scenarios.html", scope, page{"Items": data.Scenarios, "Beneficiaries": data.Beneficiaries, "Residuary": data.Residuary, "Results": data.Results})
	})
	routes.POST("/residuary", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		share, err := estateDecimal(c.PostForm("share_percent"))
		if err == nil {
			_, err = repo.UpsertEstateResiduaryShare(c, scope, parseID(c.PostForm("beneficiary_id")), share)
		}
		estateMutation(c, err, estateURL(scope)+"/scenarios")
	})
	routes.POST("/residuary/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateResiduaryShare(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/scenarios")
	})
	routes.POST("/scenarios", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		item, err := estateScenarioForm(c, uuid.Nil)
		if err == nil {
			_, err = repo.CreateEstateScenario(c, scope, item)
		}
		estateMutation(c, err, c.Request.URL.Path)
	})
	routes.POST("/scenarios/:item/update", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		item, err := estateScenarioForm(c, parseID(c.Param("item")))
		if err == nil {
			err = repo.UpdateEstateScenario(c, scope, item)
		}
		estateMutation(c, err, estateURL(scope)+"/scenarios")
	})
	routes.POST("/scenarios/:item/delete", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		estateMutation(c, repo.DeleteEstateScenario(c, scope, parseID(c.Param("item"))), estateURL(scope)+"/scenarios")
	})
	routes.POST("/scenarios/:item/run", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		scenarioID := parseID(c.Param("item"))
		input, err := repo.LoadEstateInput(c, scope, scenarioID)
		if err != nil {
			estateMutation(c, err, estateURL(scope)+"/scenarios")
			return
		}
		result, err := estate.Run(input)
		if err != nil {
			c.String(http.StatusBadRequest, "scenario failed")
			return
		}
		if !result.Valid {
			renderEstate(c, render, "estate_invalid.html", scope, page{"Errors": result.Errors})
			return
		}
		result.Readiness, err = repo.EstateReadiness(c, scope, scenarioID)
		if err != nil {
			estateMutation(c, err, estateURL(scope)+"/scenarios")
			return
		}
		resultID, err := repo.SaveEstateResult(c, scope, scenarioID, input, result)
		if err != nil {
			estateMutation(c, err, estateURL(scope)+"/scenarios")
			return
		}
		c.Redirect(http.StatusSeeOther, estateURL(scope)+"/results/"+resultID.String())
	})
	routes.GET("/results/:result", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		result, err := repo.EstateResult(c, scope, parseID(c.Param("result")))
		if err != nil {
			estateReadError(c, err)
			return
		}
		renderEstate(c, render, "estate_result.html", scope, page{"Run": result})
	})
	routes.GET("/results/:result/print", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		result, err := repo.EstateResult(c, scope, parseID(c.Param("result")))
		if err != nil {
			estateReadError(c, err)
			return
		}
		renderEstate(c, render, "estate_print.html", scope, page{"Run": result})
	})
}

func loadEstateData(c *gin.Context, repo *app.Repo) (app.Scope, app.EstateData, bool) {
	scope, ok := scopeFor(c, repo)
	if !ok {
		return app.Scope{}, app.EstateData{}, false
	}
	data, err := repo.EstateData(c, scope)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return app.Scope{}, app.EstateData{}, false
	}
	return scope, data, true
}

func renderEstate(c *gin.Context, render func(*gin.Context, string, page), name string, scope app.Scope, data page) {
	data["HouseholdID"] = scope.HouseholdID.String()
	data["EstateBase"] = estateURL(scope)
	data["Disclaimer"] = estateDisclaimer
	render(c, name, data)
}

func estateMutation(c *gin.Context, err error, redirect string) {
	if err == nil {
		c.Redirect(http.StatusSeeOther, redirect)
		return
	}
	if errors.Is(err, app.ErrNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if errors.Is(err, app.ErrCredentialDetected) {
		c.String(http.StatusBadRequest, "digital metadata must not contain credentials")
		return
	}
	c.String(http.StatusBadRequest, "invalid estate record")
}

func estateReadError(c *gin.Context, err error) {
	if errors.Is(err, app.ErrNotFound) {
		c.Status(http.StatusNotFound)
	} else {
		c.Status(http.StatusInternalServerError)
	}
}
func estateURL(scope app.Scope) string {
	return "/households/" + scope.HouseholdID.String() + "/estate"
}
func estateDecimal(value string) (decimal.Decimal, error) {
	parsed, err := decimal.NewFromString(value)
	if err != nil || parsed.IsNegative() {
		return decimal.Zero, errors.New("invalid non-negative decimal")
	}
	return parsed, nil
}
func estateDocumentForm(c *gin.Context, id uuid.UUID) app.EstateDocument {
	return app.EstateDocument{ID: id, DocumentType: c.PostForm("document_type"), StorageLocation: c.PostForm("storage_location"), Status: c.PostForm("status"), Notes: c.PostForm("notes")}
}
func estateContactForm(c *gin.Context, id uuid.UUID) app.EstateContact {
	return app.EstateContact{ID: id, Role: c.PostForm("role"), Name: c.PostForm("name"), ContactInfo: c.PostForm("contact_info"), Notes: c.PostForm("notes")}
}
func estateDigitalForm(c *gin.Context, id uuid.UUID) app.EstateDigitalAsset {
	return app.EstateDigitalAsset{ID: id, ServiceName: c.PostForm("service_name"), AccountIdentifier: c.PostForm("account_identifier"), AccessInstructionsLocation: c.PostForm("access_instructions_location"), Notes: c.PostForm("notes")}
}
func estateScenarioForm(c *gin.Context, id uuid.UUID) (app.EstateScenario, error) {
	asOfYear, err := strconv.Atoi(c.PostForm("as_of_year"))
	if err != nil {
		return app.EstateScenario{}, err
	}
	deathYear, err := strconv.Atoi(c.PostForm("death_year"))
	if err != nil {
		return app.EstateScenario{}, err
	}
	debt, err := estateDecimal(c.PostForm("debt"))
	if err != nil {
		return app.EstateScenario{}, err
	}
	funeralCost, err := estateDecimal(c.PostForm("funeral_cost"))
	if err != nil {
		return app.EstateScenario{}, err
	}
	legalCost, err := estateDecimal(c.PostForm("legal_cost"))
	if err != nil {
		return app.EstateScenario{}, err
	}
	return app.EstateScenario{ID: id, Name: c.PostForm("name"), DecedentName: c.PostForm("decedent_name"), AsOfYear: asOfYear, DeathYear: deathYear, Debt: debt, FuneralCost: funeralCost, LegalCost: legalCost, PayDebts: c.PostForm("pay_debts") == "true"}, nil
}
