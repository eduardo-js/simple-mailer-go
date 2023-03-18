package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfig_AddDefaultData(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	ctx := getCtx(r)

	r = r.WithContext(ctx)

	testApp.Session.Put(ctx, "flash", "flash")
	testApp.Session.Put(ctx, "warning", "warning")
	testApp.Session.Put(ctx, "error", "error")

	td := testApp.AddDefaultData(&TemplateData{}, r)

	if td.Flash != "flash" {
		t.Error("Failed to add flash message to default data")
	}
	if td.Warning != "warning" {
		t.Error("Failed to add warning message to default data")
	}
	if td.Error != "error" {
		t.Error("Failed to add error message to default data")
	}
}

func TestConfig_IsAuthenticated(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	ctx := getCtx(r)
	r = r.WithContext(ctx)

	var auth bool
	auth = testApp.IsAuthenticated(r)
	if auth {
		t.Error("Authenticated returned true when no session data present")
	}

	testApp.Session.Put(ctx, "userID", 1)

	auth = testApp.IsAuthenticated(r)
	if !auth {
		t.Error("Authenticated returned false when session data present")
	}

}

func TestConfig_Render(t *testing.T) {
	pathToTemplates = "./templates"

	w := httptest.NewRecorder()

	r, _ := http.NewRequest("GET", "/", nil)
	ctx := getCtx(r)
	r = r.WithContext(ctx)

	testApp.render(w, r, "home.page.gohtml", &TemplateData{})
	if w.Code != http.StatusOK {
		t.Error("Render did not return a 200")
	}
}
