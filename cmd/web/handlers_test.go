package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"simple-mailer-go/data"
	"strings"
	"testing"
)

var pageTests = []struct {
	name         string
	url          string
	expectedCode int
	handler      http.HandlerFunc
	sessionData  map[string]any
	expectHTML   string
}{
	{
		name:         "home",
		url:          "/",
		expectedCode: http.StatusOK,
		handler:      testApp.HomePage,
		expectHTML:   "Home",
	},
	{
		name:         "login",
		url:          "/login",
		expectedCode: http.StatusOK,
		handler:      testApp.LoginPage,
		expectHTML:   `<h1 class="mt-5">Login</h1>`,
	},
	{
		name:         "logout",
		url:          "/logout",
		expectedCode: http.StatusSeeOther,
		handler:      testApp.Logout,
		sessionData: map[string]any{
			"userID": 1,
			"user":   data.User{},
		},
	},
}

func Test_Pages(t *testing.T) {
	pathToTemplates = "./templates"

	for _, e := range pageTests {
		w := httptest.NewRecorder()

		r, _ := http.NewRequest("GET", e.url, nil)
		ctx := getCtx(r)
		r = r.WithContext(ctx)

		if len(e.sessionData) > 0 {
			for k, v := range e.sessionData {
				testApp.Session.Put(ctx, k, v)
			}
		}
		e.handler.ServeHTTP(w, r)

		if w.Code != e.expectedCode {
			t.Errorf("%s: expected status code %d, got %d", e.name, e.expectedCode, w.Code)
		}

		if len(e.expectHTML) > 0 {
			html := w.Body.String()
			if !strings.Contains(html, e.expectHTML) {
				t.Errorf("%s failed: expected to find %s in the response HTML", e.name, e.expectHTML)
			}
		}
	}

}

func TestConfig_PostLoginPage(t *testing.T) {
	pathToTemplates = "./templates"
	postedData := url.Values{
		"email":    {"admin@example.com"},
		"password": {"password"},
	}
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("POST", "/login", strings.NewReader(postedData.Encode()))
	ctx := getCtx(r)
	r = r.WithContext(ctx)

	handler := http.HandlerFunc(testApp.PostLoginPage)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected status code %d, got %d", http.StatusSeeOther, w.Code)
	}

	if !testApp.Session.Exists(ctx, "userID") {
		t.Errorf("expected session to exist for 'userID'")
	}
}

func TestConfig_SubscribeToPlan(t *testing.T) {
	w := httptest.NewRecorder()
	r, _ := http.NewRequest("GET", "/members/subscribe?id=1", nil)
	ctx := getCtx(r)
	r = r.WithContext(ctx)
	testApp.Session.Put(ctx, "user", data.User{
		ID:        1,
		Email:     "Admin",
		FirstName: "Admin",
		LastName:  "test",
		Active:    1,
	})

	handler := http.HandlerFunc(testApp.SubscribeToPlan)
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Errorf("expected status code %d, got %d", http.StatusSeeOther, w.Code)
	}

}
