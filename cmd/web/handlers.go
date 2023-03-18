package main

import (
	"fmt"
	"html/template"
	"net/http"
	"simple-mailer-go/data"
	"strconv"

	"github.com/phpdave11/gofpdf"
	"github.com/phpdave11/gofpdf/contrib/gofpdi"
)

func (app *Config) HomePage(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "home.page.gohtml", nil)
}

func (app *Config) LoginPage(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "login.page.gohtml", nil)
}

func (app *Config) PostLoginPage(w http.ResponseWriter, r *http.Request) {
	_ = app.Session.RenewToken(r.Context())

	err := r.ParseForm()
	if err != nil {
		app.ErrorLog.Println(err)
	}

	email := r.Form.Get("email")
	password := r.Form.Get("password")

	user, err := app.Models.User.GetByEmail(email)
	if err != nil {
		app.Session.Put(r.Context(), "error", "Invalid credentials.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	validPassword, err := user.PasswordMatches(password)
	if !validPassword {
		msg := Message{
			To:      email,
			Subject: "Failed log in attempt",
			Data:    "Invalid login attempt!",
		}

		app.sendEmail(msg)
	}
	if err != nil || !validPassword {
		app.Session.Put(r.Context(), "error", "Invalid credentials.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app.Session.Put(r.Context(), "userID", user.ID)
	app.Session.Put(r.Context(), "user", user)
	app.Session.Put(r.Context(), "flash", "Successful login!")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Config) Logout(w http.ResponseWriter, r *http.Request) {
	_ = app.Session.Destroy(r.Context())
	_ = app.Session.RenewToken(r.Context())
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Config) RegisterPage(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "register.page.gohtml", nil)
}

func (app *Config) PostRegisterPage(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()

	if err != nil {
		app.ErrorLog.Println(err)
	}
	u := data.User{
		Email:     r.Form.Get("email"),
		FirstName: r.Form.Get("first-name"),
		LastName:  r.Form.Get("last-name"),
		Password:  r.Form.Get("password"),
		Active:    0,
		IsAdmin:   0,
	}

	_, err = u.Insert(u)

	if err != nil {
		app.Session.Put(r.Context(), "error", "Failed to register user.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
	}

	url := fmt.Sprintf("http://localhost:3000/activate?email=%s", u.Email)
	signedUrl := GenerateTokenFromString(url)
	app.InfoLog.Println(signedUrl)

	msg := Message{
		To:       u.Email,
		Subject:  "Activate your account",
		Template: "confirmation-email",
		Data:     template.HTML(signedUrl),
	}

	app.sendEmail(msg)

	app.Session.Put(r.Context(), "flash", "Successful registration! Please check your email to activate your account.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (app *Config) ActivateAccount(w http.ResponseWriter, r *http.Request) {
	url := r.RequestURI
	testURL := fmt.Sprintf("http://localhost:3000%s", url)
	ok := VerifyToken(testURL)

	if !ok {
		app.Session.Put(r.Context(), "error", "Invalid activation link.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	u, err := app.Models.User.GetByEmail(r.URL.Query().Get("email"))
	if err != nil {
		app.Session.Put(r.Context(), "error", "User not found.")
		http.Redirect(w, r, "/", http.StatusNotFound)
		return
	}
	u.Active = 1
	err = u.Update()

	if err != nil {
		app.Session.Put(r.Context(), "error", "Unable to update user.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	app.Session.Put(r.Context(), "flash", "Successfully activated account. Please login.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (app *Config) ChooseSubscription(w http.ResponseWriter, r *http.Request) {
	plans, err := app.Models.Plan.GetAll()
	if err != nil {
		app.ErrorLog.Println(err)
		return
	}

	dataMap := make(map[string]any)
	dataMap["plans"] = plans
	app.render(w, r, "plans.page.gohtml", &TemplateData{
		Data: dataMap,
	})
}

func (app *Config) SubscribeToPlan(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	planID, _ := strconv.Atoi(id)
	plan, err := app.Models.Plan.GetOne(planID)
	if err != nil {
		app.Session.Put(r.Context(), "error", "Unable to find plan.")
		http.Redirect(w, r, "/members/plans", http.StatusSeeOther)
		return
	}

	user, ok := app.Session.Get(r.Context(), "user").(data.User)
	if !ok {
		app.Session.Put(r.Context(), "error", "Log in first!")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	app.Wait.Add(1)
	go func() {
		defer app.Wait.Done()

		invoice, err := app.getInvoice(user, plan)
		if err != nil {
			app.ErrorChan <- err
		}

		msg := Message{
			To:       user.Email,
			Subject:  "Your invoice plan",
			Data:     invoice,
			Template: "invoice",
		}

		app.sendEmail(msg)
	}()
	app.Wait.Add(1)
	go func() {
		defer app.Wait.Done()

		pdf := app.generateManual(user, plan)
		err := pdf.OutputFileAndClose(fmt.Sprintf("./tmp/%d_manual.pdf", user.ID))
		if err != nil {
			app.ErrorChan <- err
			return
		}
		msg := Message{
			To:      user.Email,
			Subject: "Your manual",
			Data:    "Your user manual is attached",
			AttachmentMap: map[string]string{
				"Manual.pdf": fmt.Sprintf("./tmp/%d_manual.pdf", user.ID),
			},
		}
		app.sendEmail(msg)
	}()

	err = app.Models.Plan.SubscribeUserToPlan(user, *plan)
	if err != nil {
		app.Session.Put(r.Context(), "error", "Error subscribing to plan")
		http.Redirect(w, r, "/members/plans", http.StatusSeeOther)
		return
	}
	app.Session.Put(r.Context(), "flash", "Subscribed successfully!")
	u, err := app.Models.User.GetOne(user.ID)
	if err != nil {
		app.Session.Put(r.Context(), "error", "Error getting user from database!")
		http.Redirect(w, r, "/members/plans", http.StatusSeeOther)
		return
	}
	app.Session.Put(r.Context(), "user", u)
	http.Redirect(w, r, "/members/plans", http.StatusSeeOther)
}

func (app *Config) getInvoice(u data.User, plan *data.Plan) (string, error) {
	return plan.PlanAmountFormatted, nil
}

func (app *Config) generateManual(u data.User, plan *data.Plan) *gofpdf.Fpdf {
	pdf := gofpdf.New("P", "mm", "Letter", "")
	pdf.SetMargins(10, 13, 10)
	importer := gofpdi.NewImporter()

	t := importer.ImportPage(pdf, "./pdf/manual.pdf", 1, "/MediaBox")
	pdf.AddPage()
	importer.UseImportedTemplate(pdf, t, 0, 0, 215.9, 0)
	pdf.SetX(75)
	pdf.SetY(150)
	pdf.SetFont("Arial", "", 12)

	pdf.MultiCell(0, 4, fmt.Sprintf("%s %s", u.FirstName, u.LastName), "", "C", false)
	pdf.Ln(5)
	pdf.MultiCell(0, 4, fmt.Sprintf("%s User Guide", plan.PlanName), "", "C", false)
	return pdf
}
