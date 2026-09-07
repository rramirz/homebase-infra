package app

import "testing"

func TestTemplatesIncludeEstateViews(t *testing.T) {
	templates := Templates()
	for _, name := range []string{"estate_dashboard.html", "estate_beneficiaries.html", "estate_assets.html", "estate_insurance.html", "estate_documents.html", "estate_contacts.html", "estate_digital.html", "estate_scenarios.html", "estate_result.html", "estate_print.html", "estate_invalid.html"} {
		if templates.Lookup(name) == nil {
			t.Fatalf("missing template %s", name)
		}
	}
}
