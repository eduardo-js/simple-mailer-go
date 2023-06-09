package main

import (
	"database/sql"
	"encoding/gob"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"simple-mailer-go/data"
	"sync"
	"syscall"
	"time"

	"github.com/alexedwards/scs/redisstore"
	"github.com/alexedwards/scs/v2"
	"github.com/gomodule/redigo/redis"
	_ "github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/v4"
	_ "github.com/jackc/pgx/v4/stdlib"
)

var port = os.Getenv("PORT")

func main() {
	db := initDB()
	db.Ping()
	session := initSession()
	infoLog := log.New(os.Stdout, "INFO\t", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stdout, "ERROR\t", log.Ldate|log.Ltime|log.Lshortfile)

	wg := sync.WaitGroup{}

	app := Config{
		Session:   session,
		DB:        db,
		InfoLog:   infoLog,
		ErrorLog:  errorLog,
		Wait:      &wg,
		Models:    data.New(db),
		ErrorChan: make(chan error),
		DoneChan:  make(chan bool),
	}
	app.Mailer = app.createMail()
	go app.listenForMail()
	go app.listenForErrors()
	go app.listenForShutdown()

	app.serve()
}

func (app *Config) listenForErrors() {
	for {
		select {
		case err := <-app.ErrorChan:
			app.ErrorLog.Println(err)
		case <-app.DoneChan:
			return
		}
	}
}

func initDB() *sql.DB {
	conn := connectToDB()
	if conn == nil {
		log.Panic("can't connect to database")
	}
	return conn
}

func connectToDB() *sql.DB {
	dsn := os.Getenv("DSN")
	for counts := 0; counts < 10; counts++ {
		connection, err := openDB(dsn)
		if err != nil {
			log.Println("Failed to connect to DB, retrying in 1 second")
			time.Sleep(1 * time.Second)
			continue
		} else {
			log.Print("Connected to DB!")
			return connection
		}
	}
	return nil
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func initSession() *scs.SessionManager {
	gob.Register(data.User{})
	session := scs.New()
	session.Store = redisstore.New(initRedis())
	session.Lifetime = 24 * time.Hour
	session.Cookie.Persist = true
	session.Cookie.SameSite = http.SameSiteLaxMode
	session.Cookie.Secure = true

	return session
}

func initRedis() *redis.Pool {
	redisPool := &redis.Pool{
		MaxIdle: 10,
		Dial: func() (redis.Conn, error) {
			return redis.Dial("tcp", os.Getenv("REDIS"))
		},
	}

	return redisPool
}

func (app *Config) serve() {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", port),
		Handler: app.routes(),
	}

	app.InfoLog.Println("Starting web server...")
	err := srv.ListenAndServe()
	if err != nil {
		log.Panic(err)
	}
}

func (app *Config) listenForShutdown() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	app.shutdown()
	os.Exit(0)
}

func (app *Config) shutdown() {
	app.InfoLog.Println("Waiting for shutdown...")

	app.Wait.Wait()
	app.Mailer.DoneChan <- true
	app.DoneChan <- true

	close(app.Mailer.MailerChan)
	close(app.Mailer.DoneChan)
	close(app.Mailer.ErrorChan)
	close(app.DoneChan)
	app.InfoLog.Println("Shutting down.")
}

func (app *Config) createMail() Mail {
	return Mail{
		Domain:      "localhost",
		Host:        "localhost",
		Port:        1025,
		Encryption:  "none",
		FromName:    "Info",
		FromAddress: "info@localhost",
		MailerChan:  make(chan Message, 100),
		ErrorChan:   make(chan error),
		DoneChan:    make(chan bool),
		Wait:        app.Wait,
	}
}

func (app *Config) sendEmail(msg Message) {
	app.Mailer.MailerChan <- msg
}
