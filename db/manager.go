package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jonreesman/watch-dog-kafka/twitter"
)

// Defines our Database Manager that controls
// how an external package would implement this
// package. It contains our DB connection as well
// as important database information.
type DBManager struct {
	db     *sql.DB
	dbName string
	dbUser string
	dbPwd  string
	dbURL  string
	URI    string
}

type TickerSlice []Ticker

//Defines a ticker object packaged for pushing to a database.
type Ticker struct {
	Name            string
	LastScrapeTime  time.Time
	numTweets       int
	Tweets          []twitter.Statement
	HourlySentiment float64
	Id              int
	Active          int
}

func (dbManager DBManager) Close() {
	dbManager.db.Close()
}

// Creates and returns a DB Manager based off the type
// of connection the implementer needs. Here, it is either
// a master or slave connection.
func NewManager(dbUser, dbPwd, dbName, dbURL string) (DBManager, error) {
	var (
		d   DBManager
		err error
	)
	d.dbUser = dbUser
	d.dbPwd = dbPwd
	d.dbName = dbName
	d.dbURL = dbURL

	d.URI = fmt.Sprintf("%s:%s@tcp(%s)/%s", d.dbUser, d.dbPwd, d.dbURL, d.dbName)
	d.db, err = sql.Open("mysql", d.URI)
	if err != nil {
		log.Printf("Failed to open connection in DB NewManager(): %v", err)
		return DBManager{}, err
	}

	if err := d.db.Ping(); err != nil {
		log.Printf("Failed to Ping DB in NewManager(): %v", err)
		return DBManager{}, err
	}

	if _, err := d.db.Exec(fmt.Sprintf("USE %s", d.dbName)); err != nil {
		log.Printf("Failed to `USE %s` in NewManager(): %v", d.dbName, err)
		return DBManager{}, err
	}

	d.db.SetMaxOpenConns(55)
	d.db.SetMaxIdleConns(55)

	// ReturnActiveTickers() is used as a test here
	// to see if the tables have already been created.
	// If not, the DB driver will create them automatically.
	/*if _, err := d.ReturnActiveTickers(); err != nil {
		d.createTickerTable()
		d.createStatementTable()
		d.createSentimentTable()
	}*/
	return d, nil
}
