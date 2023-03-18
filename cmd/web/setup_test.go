package main

import (
	"context"
	"encoding/gob"
	"log"
	"net/http"
	"os"
	"simple-mailer-go/data"
	"sync"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
)

var testApp Config

func TestMain(m *testing.M) {
	gob.Register(data.User{})
	gob.Register(data.User{})

	tmpPath = "./../../tmp"
	pathToManual = "./../../pdf"
	session := scs.New()
	session.Lifetime = 24 * time.Hour
	session.Cookie.Persist = true
	session.Cookie.SameSite = http.SameSiteLaxMode
	session.Cookie.Secure = true

	wait := &sync.WaitGroup{}
	testApp = Config{
		Session:   session,
		DB:        nil,
		InfoLog:   log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime),
		ErrorLog:  log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile),
		Wait:      wait,
		ErrorChan: make(chan error),
		DoneChan:  make(chan bool),
		Models:    data.TestNew(nil),
		Mailer: Mail{
			Wait:       wait,
			DoneChan:   make(chan bool),
			ErrorChan:  make(chan error),
			MailerChan: make(chan Message, 100),
		},
	}

	go func() {
		for {
			select {
			case <-testApp.Mailer.MailerChan:
				testApp.Wait.Done()
			case <-testApp.Mailer.ErrorChan:
			case <-testApp.Mailer.DoneChan:
				return
			}
		}
	}()

	go func() {
		for {
			select {
			case err := <-testApp.ErrorChan:
				testApp.ErrorLog.Println(err)
			case <-testApp.DoneChan:
				return
			}
		}
	}()

	os.Exit(m.Run())
}

func getCtx(r *http.Request) context.Context {
	ctx, err := testApp.Session.Load(r.Context(), r.Header.Get("X-Session"))
	if err != nil {
		log.Println(err)
	}
	return ctx
}
