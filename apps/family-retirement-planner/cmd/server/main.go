package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"

	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/internal/app"
	"github.com/rramirz/homebase-infra/apps/family-retirement-planner/pkg/retirement"
	"github.com/shopspring/decimal"
)

type page map[string]any

func main() {
	ctx := context.Background()
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	repo := app.NewRepo(db)
	if err := repo.Migrate(ctx); err != nil {
		panic(err)
	}
	secure := os.Getenv("COOKIE_SECURE") == "true"
	r := gin.New()
	r.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		c.Status(http.StatusInternalServerError)
	}), headers(), csrf(), requestLimit())
	r.GET("/health/live", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/health/ready", func(c *gin.Context) {
		if err := db.Ping(c); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.StaticFS("/static", http.FS(app.StaticAssets()))
	r.GET("/favicon.ico", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	t := app.Templates()
	render := func(c *gin.Context, n string, p page) {
		p["CSRF"] = c.GetString("csrf")
		if v, ok := c.Get("household"); ok {
			p["Household"] = v
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		if err := t.ExecuteTemplate(c.Writer, n, p); err != nil {
			c.String(500, "Internal Server Error")
		}
	}
	r.GET("/register", func(c *gin.Context) {
		render(c, "auth", page{"Title": "Register", "Other": "Login", "OtherURL": "/login"})
	})
	r.POST("/register", func(c *gin.Context) {
		_, err := repo.Register(c, c.PostForm("email"), c.PostForm("password"))
		if err != nil {
			render(c, "auth", page{"Title": "Register", "Error": "Registration failed", "Other": "Login", "OtherURL": "/login"})
			return
		}
		c.Redirect(303, "/login")
	})
	r.GET("/login", func(c *gin.Context) {
		render(c, "auth", page{"Title": "Login", "Other": "Register", "OtherURL": "/register"})
	})
	r.POST("/login", func(c *gin.Context) {
		_, token, err := repo.Login(c, c.PostForm("email"), c.PostForm("password"))
		if err != nil {
			render(c, "auth", page{"Title": "Login", "Error": "Invalid credentials", "Other": "Register", "OtherURL": "/register"})
			return
		}
		setCookie(c, "session", token, 14*86400, secure)
		c.Redirect(303, "/households")
	})
	r.POST("/logout", func(c *gin.Context) {
		if token, e := sessionCookie(c); e == nil {
			_ = repo.Logout(c, token)
		}
		setCookie(c, "session", "", -1, secure)
		c.Redirect(303, "/login")
	})
	auth := func(c *gin.Context) {
		token, e := sessionCookie(c)
		if e != nil {
			c.Redirect(303, "/login")
			c.Abort()
			return
		}
		u, e := repo.UserForSession(c, token)
		if e != nil {
			c.Redirect(303, "/login")
			c.Abort()
			return
		}
		c.Set("user", u)
	}
	private := r.Group("", auth)
	registerEstateRoutes(private, repo, render)
	private.GET("/", func(c *gin.Context) { c.Redirect(303, "/households") })
	private.GET("/households", func(c *gin.Context) {
		hs, e := repo.Households(c, user(c))
		if e != nil {
			c.Status(500)
			return
		}
		render(c, "households.html", page{"Items": hs})
	})
	private.POST("/households/:id/update", func(c *gin.Context) {
		if e := repo.UpdateHousehold(c, user(c), parseID(c.Param("id")), c.PostForm("name")); e != nil {
			c.Status(400)
			return
		}
		c.Redirect(303, "/households/"+c.Param("id"))
	})
	private.POST("/households", func(c *gin.Context) {
		id, e := repo.CreateHousehold(c, user(c), c.PostForm("name"))
		if e != nil {
			c.Status(400)
			return
		}
		c.Redirect(303, "/households/"+id.String())
	})
	private.POST("/households/:id/archive", func(c *gin.Context) {
		_ = repo.ArchiveHousehold(c, user(c), parseID(c.Param("id")))
		c.Redirect(303, "/households")
	})
	private.GET("/households/:id", func(c *gin.Context) {
		scope, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		plans, e := repo.Plans(c, scope)
		if e != nil {
			c.Status(500)
			return
		}
		links := nav(c.Param("id"))
		if len(plans) == 0 {
			render(c, "resources.html", page{"Title": "Plans", "Links": links, "Kinds": []string{}, "AmountLabel": "", "Items": []app.Resource{}})
			return
		}
		showDashboard(c, repo, render, scope, plans[0], links)
	})
	for _, table := range []string{"people", "accounts", "liabilities", "incomes", "expenses"} {
		table := table
		private.GET("/households/:id/"+table, func(c *gin.Context) {
			s, ok := scopeFor(c, repo)
			if !ok {
				return
			}
			xs, e := repo.ListResources(c, s, table)
			if e != nil {
				c.Status(500)
				return
			}
			meta := resourceMeta(table)
			render(c, "resources.html", page{"Title": table, "Items": xs, "Links": nav(c.Param("id")), "DeletePrefix": "/households/" + c.Param("id") + "/" + table, "Kinds": meta.kinds, "AmountLabel": meta.label, "Years": meta.years, "Rate": meta.rate})
		})
		private.POST("/households/:id/"+table, func(c *gin.Context) {
			s, ok := scopeFor(c, repo)
			if !ok {
				return
			}
			x := app.Resource{Name: c.PostForm("name"), Kind: c.PostForm("kind"), Amount: c.PostForm("amount"), StartYear: c.PostForm("start_year"), EndYear: c.PostForm("end_year"), Inflation: c.PostForm("inflation"), Confidence: c.PostForm("confidence")}
			if table == "liabilities" {
				x.Inflation = c.PostForm("inflation")
				x.EndYear = c.PostForm("end_year")
			}
			if e := repo.CreateResource(c, s, table, x); e != nil {
				c.String(400, "invalid resource")
				return
			}
			c.Redirect(303, c.Request.URL.Path)
		})
		private.POST("/households/:id/"+table+"/:resource/delete", func(c *gin.Context) {
			s, ok := scopeFor(c, repo)
			if !ok {
				return
			}
			_ = repo.DeleteResource(c, s, table, parseID(c.Param("resource")))
			c.Redirect(303, "/households/"+c.Param("id")+"/"+table)
		})
		private.POST("/households/:id/"+table+"/:resource/update", func(c *gin.Context) {
			s, ok := scopeFor(c, repo)
			if !ok {
				return
			}
			x := app.Resource{
				ID:         parseID(c.Param("resource")),
				Name:       c.PostForm("name"),
				Kind:       c.PostForm("kind"),
				Amount:     c.PostForm("amount"),
				StartYear:  c.PostForm("start_year"),
				EndYear:    c.PostForm("end_year"),
				Inflation:  c.PostForm("inflation"),
				Confidence: c.PostForm("confidence"),
			}
			if table == "liabilities" {
				x.Inflation = c.PostForm("inflation")
				x.EndYear = c.PostForm("end_year")
			}
			if e := repo.UpdateResource(c, s, table, x); e != nil {
				c.String(400, "invalid resource")
				return
			}
			c.Redirect(303, "/households/"+c.Param("id")+"/"+table)
		})
	}
	private.GET("/households/:id/plans", func(c *gin.Context) {
		s, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		ps, e := repo.Plans(c, s)
		if e != nil {
			c.Status(500)
			return
		}
		render(c, "plans.html", page{"Title": "Retirement plans", "Items": ps, "Links": nav(c.Param("id")), "HouseholdID": c.Param("id")})
	})
	private.POST("/households/:id/plans", func(c *gin.Context) {
		s, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		a, _ := strconv.Atoi(c.PostForm("as_of_year"))
		z, _ := strconv.Atoi(c.PostForm("end_year"))
		id, e := repo.CreatePlan(c, s, app.Plan{Name: c.PostForm("name"), AsOfYear: a, EndYear: z, AnnualReturn: c.PostForm("annual_return"), Inflation: c.PostForm("inflation"), Confidence: c.PostForm("confidence")})
		if e != nil {
			c.String(400, "invalid plan")
			return
		}
		c.Redirect(303, "/households/"+c.Param("id")+"/plans/"+id.String())
	})
	private.POST("/households/:id/plans/:plan/update", func(c *gin.Context) {
		s, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		a, _ := strconv.Atoi(c.PostForm("as_of_year"))
		z, _ := strconv.Atoi(c.PostForm("end_year"))
		if e := repo.UpdatePlan(c, s, app.Plan{ID: parseID(c.Param("plan")), Name: c.PostForm("name"), AsOfYear: a, EndYear: z, AnnualReturn: c.PostForm("annual_return"), Inflation: c.PostForm("inflation"), Confidence: c.PostForm("confidence")}); e != nil {
			c.String(400, "invalid plan")
			return
		}
		c.Redirect(303, "/households/"+c.Param("id")+"/plans")
	})
	private.GET("/households/:id/plans/:plan", func(c *gin.Context) {
		s, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		ps, _ := repo.Plans(c, s)
		for _, p := range ps {
			if p.ID == parseID(c.Param("plan")) {
				showDashboard(c, repo, render, s, p, nav(c.Param("id")))
				return
			}
		}
		c.Status(404)
	})
	private.GET("/households/:id/plans/:plan/scenarios", func(c *gin.Context) {
		s, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		xs, e := repo.Scenarios(c, s, parseID(c.Param("plan")))
		if e != nil {
			c.Status(500)
			return
		}
		ps, _ := repo.Plans(c, s)
		for _, p := range ps {
			if p.ID == parseID(c.Param("plan")) {
				render(c, "scenarios.html", page{"Plan": p, "Items": xs, "CloneURL": c.Request.URL.Path, "RunURL": c.Request.URL.Path, "HouseholdID": s.HouseholdID.String()})
				return
			}
		}
		c.Status(404)
	})

	private.POST("/households/:id/plans/:plan/scenarios/:scenario/update", func(c *gin.Context) {
		s, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		if e := repo.UpdateScenario(c, s, app.Scenario{ID: parseID(c.Param("scenario")), Name: c.PostForm("name"), Overrides: []byte(c.PostForm("overrides"))}); e != nil {
			c.Status(400)
			return
		}
		c.Redirect(303, "/households/"+c.Param("id")+"/plans/"+c.Param("plan")+"/scenarios")
	})

	private.GET("/households/:id/plans/:plan/compare", func(c *gin.Context) {
		s, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		ps, _ := repo.Plans(c, s)
		var plan app.Plan
		for _, p := range ps {
			if p.ID == parseID(c.Param("plan")) {
				plan = p
				break
			}
		}
		if plan.ID == uuid.Nil {
			c.Status(404)
			return
		}

		scenarios, _ := repo.Scenarios(c, s, plan.ID)
		incomes, _ := repo.ListResources(c, s, "incomes")
		expenses, _ := repo.ListResources(c, s, "expenses")

		type CompareRow struct {
			Name          string
			AnnualReturn  string
			Inflation     string
			EndingAssets  string
			DepletionYear int
		}
		var rows []CompareRow

		compute := func(name string, overrides []byte) {
			input := projectionInput(c, repo, s, plan)
			app.ApplyOverrides(&input, overrides, incomes, expenses)
			proj, _ := retirement.Project(input)
			endAssets := "0"
			if len(proj.Years) > 0 {
				endAssets = proj.Years[len(proj.Years)-1].EndingPortfolio.StringFixed(0)
			}
			rows = append(rows, CompareRow{
				Name: name, AnnualReturn: input.Assumptions.AnnualReturn.StringFixed(4),
				Inflation:    input.Assumptions.Inflation.StringFixed(4),
				EndingAssets: endAssets, DepletionYear: proj.DepletionYear,
			})
		}

		compute("Baseline", nil)
		for _, sc := range scenarios {
			compute(sc.Name, sc.Overrides)
		}

		render(c, "compare.html", page{"Plan": plan, "Rows": rows, "Links": nav(c.Param("id"))})
	})

	private.POST("/households/:id/plans/:plan/scenarios", func(c *gin.Context) {
		s, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		id, e := repo.CreateScenario(c, s, parseID(c.Param("plan")), c.PostForm("name"), []byte(c.PostForm("overrides")))
		if e != nil {
			c.String(400, "invalid scenario")
			return
		}
		c.Redirect(303, c.Request.URL.Path+"/"+id.String())
	})
	private.POST("/households/:id/plans/:plan/scenarios/:scenario/clone", func(c *gin.Context) {
		s, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		id, e := repo.CloneScenario(c, s, parseID(c.Param("scenario")), c.PostForm("name"))
		if e != nil {
			c.Status(404)
			return
		}
		c.Redirect(303, "/households/"+c.Param("id")+"/plans/"+c.Param("plan")+"/scenarios/"+id.String())
	})
	private.GET("/households/:id/plans/:plan/scenarios/:scenario", func(c *gin.Context) {
		s, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		ps, _ := repo.Plans(c, s)
		for _, p := range ps {
			if p.ID == parseID(c.Param("plan")) {
				xs, _ := repo.Scenarios(c, s, p.ID)
				for _, x := range xs {
					if x.ID == parseID(c.Param("scenario")) {
						showDashboardOverride(c, repo, render, s, p, nav(c.Param("id")), x.Overrides)
						return
					}
				}
			}
		}
		c.Status(404)
	})
	private.GET("/households/:id/plans/:plan/year/:year", func(c *gin.Context) {
		s, ok := scopeFor(c, repo)
		if !ok {
			return
		}
		ps, _ := repo.Plans(c, s)
		for _, p := range ps {
			if p.ID == parseID(c.Param("plan")) {
				input := projectionInput(c, repo, s, p)
				out, e := retirement.Project(input)
				if e != nil {
					c.Status(400)
					return
				}
				y, _ := strconv.Atoi(c.Param("year"))
				for _, row := range out.Years {
					if row.Year == y {
						render(c, "year.html", page{"Year": row, "Back": "/households/" + c.Param("id") + "/plans/" + c.Param("plan")})
						return
					}
				}
			}
		}
		c.Status(404)
	})
	server := &http.Server{Addr: env("ADDR", ":8080"), Handler: r, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func user(c *gin.Context) uuid.UUID { return c.MustGet("user").(uuid.UUID) }
func parseID(s string) uuid.UUID    { id, _ := uuid.Parse(s); return id }
func headers() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; style-src 'self'")
		c.Header("Cache-Control", "no-store, private")
		c.Header("Pragma", "no-cache")
		c.Next()
	}
}
func requestLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
		c.Next()
	}
}
func csrf() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, e := csrfCookie(c)
		if e != nil {
			b := make([]byte, 32)
			if _, err := rand.Read(b); err != nil {
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}
			token = hex.EncodeToString(b)
			setCookie(c, "csrf", token, 86400, os.Getenv("COOKIE_SECURE") == "true")
		}
		c.Set("csrf", token)
		if c.Request.Method == "POST" {
			got := c.GetHeader("X-CSRF-Token")
			if got == "" {
				got = c.PostForm("csrf")
			}
			if subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				c.AbortWithStatus(403)
				return
			}
		}
		c.Next()
	}
}

func setCookie(c *gin.Context, name, value string, maxAge int, secure bool) {
	if secure {
		name = "__Host-" + name
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: name, Value: value, Path: "/", MaxAge: maxAge, Secure: secure, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}
func csrfCookie(c *gin.Context) (string, error) {
	if os.Getenv("COOKIE_SECURE") == "true" {
		return c.Cookie("__Host-csrf")
	}
	return c.Cookie("csrf")
}
func sessionCookie(c *gin.Context) (string, error) {
	if os.Getenv("COOKIE_SECURE") == "true" {
		return c.Cookie("__Host-session")
	}
	return c.Cookie("session")
}
func scopeFor(c *gin.Context, r *app.Repo) (app.Scope, bool) {
	id := parseID(c.Param("id"))
	hs, e := r.Households(c, user(c))
	if e != nil {
		c.Status(500)
		return app.Scope{}, false
	}
	for _, h := range hs {
		if h.ID == id {
			c.Set("household", id.String())
			return app.Scope{UserID: user(c), HouseholdID: id}, true
		}
	}
	c.Status(404)
	return app.Scope{}, false
}

type meta struct {
	kinds       []string
	label       string
	years, rate bool
}

func resourceMeta(t string) meta {
	switch t {
	case "people":
		return meta{label: "Birth year", years: true}
	case "accounts":
		return meta{[]string{"cash", "taxable", "tax_deferred", "roth"}, "Balance", false, false}
	case "liabilities":
		return meta{label: "Balance", years: true, rate: true}
	case "incomes":
		return meta{[]string{"social_security", "pension", "employment", "other"}, "Annual amount", true, true}
	default:
		return meta{[]string{"essential", "lifestyle", "healthcare", "one_time"}, "Annual amount", true, true}
	}
}
func nav(id string) []map[string]string {
	base := "/households/" + id
	return []map[string]string{{"Name": "People", "URL": base + "/people"}, {"Name": "Accounts", "URL": base + "/accounts"}, {"Name": "Liabilities", "URL": base + "/liabilities"}, {"Name": "Income", "URL": base + "/incomes"}, {"Name": "Expenses", "URL": base + "/expenses"}, {"Name": "Plans", "URL": base + "/plans"}}
}
func plansAsResources(ps []app.Plan) []app.Resource {
	out := []app.Resource{}
	for _, p := range ps {
		out = append(out, app.Resource{ID: p.ID, Name: p.Name, Amount: strconv.Itoa(p.AsOfYear), StartYear: strconv.Itoa(p.EndYear)})
	}
	return out
}
func showDashboard(c *gin.Context, r *app.Repo, render func(*gin.Context, string, page), s app.Scope, p app.Plan, links []map[string]string) {
	showDashboardOverride(c, r, render, s, p, links, nil)
}

func showDashboardOverride(c *gin.Context, r *app.Repo, render func(*gin.Context, string, page), s app.Scope, p app.Plan, links []map[string]string, overrides []byte) {
	input := projectionInput(c, r, s, p)
	incomes, _ := r.ListResources(c, s, "incomes")
	expenses, _ := r.ListResources(c, s, "expenses")

	if err := app.ApplyOverrides(&input, overrides, incomes, expenses); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	projection, err := retirement.Project(input)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	var missing []string
	verifiedCount, totalCount := 0, 0
	accounts, _ := r.ListResources(c, s, "accounts")
	people, _ := r.ListResources(c, s, "people")

	if len(people) == 0 {
		missing = append(missing, "people")
	}
	if len(accounts) == 0 {
		missing = append(missing, "liquid accounts")
	}
	if len(incomes) == 0 {
		missing = append(missing, "income")
	}
	if len(expenses) == 0 {
		missing = append(missing, "expenses")
	}

	for _, list := range [][]app.Resource{accounts, people, incomes, expenses} {
		for _, it := range list {
			totalCount++
			if it.Confidence == "verified" {
				verifiedCount++
			}
			if it.Confidence == "unknown" || it.Confidence == "" {
				missing = append(missing, it.Name+" (unknown)")
			}
		}
	}
	completeness := 0
	if totalCount > 0 {
		completeness = (verifiedCount * 100) / totalCount
	}

	readiness, reason := "Strong", "No projection shortfall."
	if projection.DepletionYear > 0 {
		readiness = "High Risk"
		reason = "Liquid portfolio depletes in " + strconv.Itoa(projection.DepletionYear)
	} else if len(projection.Years) > 0 && projection.Years[len(projection.Years)-1].EndingPortfolio.LessThan(decimal.NewFromInt(1000)) {
		readiness = "Needs Attention"
		reason = "Projection ends with limited liquid assets."
	}

	points := ""
	if len(projection.Years) > 0 {
		var maxP, minP decimal.Decimal
		for i, y := range projection.Years {
			if i == 0 || y.EndingPortfolio.GreaterThan(maxP) {
				maxP = y.EndingPortfolio
			}
			if i == 0 || y.EndingPortfolio.LessThan(minP) {
				minP = y.EndingPortfolio
			}
		}
		diff := maxP.Sub(minP)
		width, height := 800.0, 200.0
		for i, y := range projection.Years {
			x := (float64(i) / float64(len(projection.Years)-1)) * width
			yVal := height
			if diff.GreaterThan(decimal.Zero) {
				fDiff, _ := diff.Float64()
				fY, _ := y.EndingPortfolio.Sub(minP).Float64()
				yVal = height - ((fY / fDiff) * height)
			}
			points += fmt.Sprintf("%.1f,%.1f ", x, yVal)
		}
	}

	render(c, "dashboard.html", page{"Plan": p, "Years": projection.Years, "Links": links, "Readiness": readiness, "ReadinessReason": reason, "Points": points, "YearURL": "/households/" + s.HouseholdID.String() + "/plans/" + p.ID.String() + "/year", "Missing": missing, "Completeness": completeness})
}
func projectionInput(c *gin.Context, r *app.Repo, s app.Scope, p app.Plan) retirement.Input {
	accounts, _ := r.ListResources(c, s, "accounts")
	incomes, _ := r.ListResources(c, s, "incomes")
	expenses, _ := r.ListResources(c, s, "expenses")
	liabilities, _ := r.ListResources(c, s, "liabilities")
	people, _ := r.ListResources(c, s, "people")
	input := retirement.Input{Assumptions: retirement.Assumptions{AsOfYear: p.AsOfYear, EndYear: p.EndYear, AnnualReturn: decimal.RequireFromString(p.AnnualReturn), Inflation: decimal.RequireFromString(p.Inflation)}}
	for _, x := range people {
		birthYear, _ := strconv.Atoi(x.Amount)
		retirementYear, _ := strconv.Atoi(x.StartYear)
		deathYear, _ := strconv.Atoi(x.EndYear)
		input.People = append(input.People, retirement.Person{ID: x.ID.String(), Name: x.Name, BirthYear: birthYear, RetirementYear: retirementYear, DeathYear: deathYear})
	}
	for _, x := range accounts {
		input.Accounts = append(input.Accounts, retirement.Account{Kind: x.Kind, Balance: decimal.RequireFromString(x.Amount)})
	}
	for _, x := range incomes {
		a, _ := strconv.Atoi(x.StartYear)
		b, _ := strconv.Atoi(x.EndYear)
		input.Incomes = append(input.Incomes, retirement.Income{ID: x.ID.String(), Name: x.Name, Kind: x.Kind, Amount: decimal.RequireFromString(x.Amount), StartYear: a, EndYear: b, Inflation: decimal.RequireFromString(x.Inflation)})
	}
	for _, x := range expenses {
		a, _ := strconv.Atoi(x.StartYear)
		b, _ := strconv.Atoi(x.EndYear)
		input.Expenses = append(input.Expenses, retirement.Expense{ID: x.ID.String(), Name: x.Name, Kind: x.Kind, Amount: decimal.RequireFromString(x.Amount), StartYear: a, EndYear: b, Inflation: decimal.RequireFromString(x.Inflation)})
	}
	for _, x := range liabilities {
		input.Liabilities = append(input.Liabilities, retirement.Liability{Name: x.Name, Balance: decimal.RequireFromString(x.Amount), AnnualRate: decimal.RequireFromString(x.Inflation), AnnualPayment: decimal.RequireFromString(x.EndYear)})
	}
	return input
}
