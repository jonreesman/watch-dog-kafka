package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jonreesman/watch-dog-kafka/twitter"
)

// Defines our Database Manager that controls
// how an external package would implement this
// package. It contains our DB connection as well
// as inportant database information.
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

func (d DBManager) Close() {
	d.db.Close()
}

type Type int

const (
	MASTER = iota
	SLAVE
)

// Creates and returns a DB Manager based off the type
// of connection the implementer needs. Here, it is either
// a master or slave connection.
func NewManager(t Type) (DBManager, error) {
	var (
		d   DBManager
		err error
	)
	d.dbUser = os.Getenv("DB_USER")
	d.dbPwd = os.Getenv("DB_PWD")
	d.dbName = os.Getenv("DB_NAME")

	if t == MASTER {
		d.dbURL = os.Getenv("DB_MASTER")
	} else {
		d.dbURL = os.Getenv("DB_SLAVE")
	}
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

	fmt.Println("Connection established")

	// ReturnActiveTickers() is used as a test here
	// to see if the tables have already been created.
	// If not, the DB driver will create them automatically.
	if _, err := d.ReturnActiveTickers(); err != nil {
		d.createTickerTable()
		d.createStatementTable()
		d.createSentimentTable()
	}
	return d, nil
}

const addSentimentQuery = `
INSERT INTO sentiments(time_stamp, ticker_id, hourly_sentiment) ` +
	`VALUES (?, ?, ?)`

// Adds an average hourly sentiment to the database.
// less concerned with any errors, it does not disrupt the application,
// but does generate an error message for diagnosing issues.
func (d DBManager) AddSentiment(wg *sync.WaitGroup, timeStamp int64, tickerId int, hourlySentiment float64) {
	defer wg.Done()
	_, err := d.db.Exec(addSentimentQuery,
		timeStamp,
		tickerId,
		float32(hourlySentiment),
	)
	if err != nil {
		log.Print("Error in addSentiment()", err, hourlySentiment)
		log.Print("id is ", tickerId)
	}
}

const activateTickerQuery = `
UPDATE tickers SET active=1 WHERE ticker_id=?`
const addTickerQuery = `
INSERT INTO tickers(name, active, last_scrape_time) ` +
	`VALUES (?,?,?)`

// Checks if a ticker already exists, then attempts to add
// the ticker to the database if not. Returns the id assigned
// to the ticker if successful, else returns 0 and an error.
// Id 0 is reserved by the program for error purposes.
func (d DBManager) AddTicker(name string) (int, error) {
	if name == "" {
		return 0, errors.New("ticker name is blank")
	}
	if t, err := d.RetrieveTickerByName(name); err == nil {
		if t.Active == 1 {
			return t.Id, errors.New("ticker already exists and is active")
		}
		if _, err := d.db.Exec(activateTickerQuery, t.Id); err != nil {
			return 0, err
		}
		return t.Id, nil
	}

	dbQuery, err := d.db.Prepare(addTickerQuery)
	if err != nil {
		log.Print("Error in AddTicker()", err)
		return 0, err
	}
	if _, err := dbQuery.Query(name, 1, nil); err != nil {
		return 0, err

	}

	id, err := d.RetrieveTickerIDByName(name)
	if err != nil {
		return 0, err
	}
	return id, nil

}

const updateTickerQuery = `
UPDATE tickers SET last_scrape_time=? ` +
	`WHERE ticker_id=?`

// Updates the last_scrape_time for a ticker
// upon completion of an hourly scrape.
func (d DBManager) UpdateTicker(wg *sync.WaitGroup, id int, timeStamp time.Time) error {
	defer wg.Done()
	if _, err := d.db.Exec(updateTickerQuery, timeStamp.Unix(), id); err != nil {
		return err
	}
	return nil
}

const addStatementQuery = `
INSERT INTO statements(ticker_id, expression, time_stamp, polarity, url, tweet_id) ` +
	`VALUES (?, ?, ?, ?, ?, ?)`

// Adds a single tweet to the statement table of the database.
func (d DBManager) AddStatement(tickerId int, expression string, timeStamp int64, polarity float64, url string, tweet_id uint64) {
	_, err := d.db.Exec(addStatementQuery,
		tickerId,
		expression,
		timeStamp,
		float32(polarity),
		url,
		tweet_id,
	)
	if err != nil {
		log.Print("Error in addStatement", err)
	}
}

const retrieveTickerByNameQuery = `
SELECT ticker_id, name, last_scrape_time, active FROM tickers ` +
	`WHERE name=?`

// Allows the user to retrieve a ticker entry from the database
// with only the ticker name. Ticker name is assumed to be
// unique, as that is true for the NASDAQ.
func (d DBManager) RetrieveTickerByName(tickerName string) (Ticker, error) {
	rows, err := d.db.Query(retrieveTickerByNameQuery, tickerName)
	if err != nil {
		log.Print(err)
	}
	defer rows.Close()
	var (
		id             int
		name           string
		lastScrapeTime sql.NullInt64
		active         int
	)
	for rows.Next() {
		err := rows.Scan(&id, &name, &lastScrapeTime, &active)
		if err != nil {
			log.Print(err)
		}
		if name != tickerName {
			continue
		}
		t := Ticker{
			Id:             id,
			Name:           name,
			LastScrapeTime: time.Unix(lastScrapeTime.Int64, 0),
			Active:         active,
		}
		return t, nil

	}
	return Ticker{Id: 0, Name: ""}, errors.New("ticker does not exist with that ID")
}

// Allows us to retrieve a ticker by ID, which is assigned
// by the database. This is mostly heavily used internally,
// as the user doesnt necessarily know tickers by their DB IDs.
func (d DBManager) RetrieveTickerIDByName(tickerName string) (int, error) {
	rows, err := d.db.Query(retrieveTickerByNameQuery, tickerName)
	if err != nil {
		log.Print(err)
	}
	defer rows.Close()
	var (
		id             int
		name           string
		lastScrapeTime sql.NullInt64
		active         int
	)
	for rows.Next() {
		// Here we just print the error, as we do not want errors
		// to prevent us from finding the ticker in rows.Scan if it
		// fails on a completely unrelated ticker/line.
		if err := rows.Scan(&id, &name, &lastScrapeTime, &active); err != nil {
			log.Print(err)
		}
		if name == tickerName {
			return id, nil
		}
	}
	return 0, errors.New("ticker does not exist with that ID")
}

// Retrieves the last scrape time for a ticker.
func (d DBManager) RetrieveTickerLastScrapeTime(tickerName string) (int64, error) {
	rows, err := d.db.Query(retrieveTickerByNameQuery, tickerName)
	if err != nil {
		log.Print(err)
	}
	defer rows.Close()
	var (
		id             int
		name           string
		lastScrapeTime sql.NullInt64
		active         int
	)
	for rows.Next() {
		// Here we just print the error, as we do not want errors
		// to prevent us from finding the ticker in rows.Scan if it
		// fails on a completely unrelated ticker/line.
		if err := rows.Scan(&id, &name, &lastScrapeTime, &active); err != nil {
			log.Print(err)
		}
		if name == tickerName {
			return lastScrapeTime.Int64, nil
		}
	}
	return 0, errors.New("ticker does not exist with that ID")
}

const activeTickerQuery = `
SELECT tickers.ticker_id, tickers.name, tickers.last_scrape_time, ` +
	`sentiments.hourly_sentiment FROM tickers LEFT JOIN sentiments ` +
	`ON tickers.ticker_id = sentiments.ticker_id ` +
	`AND tickers.last_scrape_time = sentiments.time_stamp ` +
	`WHERE active=1 ORDER BY ticker_id`

// Searches for and returns only tickers presently listed as active.
func (d DBManager) ReturnActiveTickers() (tickers TickerSlice, err error) {
	rows, err := d.db.Query(activeTickerQuery)
	if err != nil {
		log.Printf("Database Error in ReturnActiveTickers(): %v", err)
		return nil, err
	}
	defer rows.Close()

	var (
		id                    int
		name                  string
		lastScrapeTimeHolder  sql.NullInt64
		lastScrapeTime        int64
		hourlySentimentHolder sql.NullFloat64
		hourlySentiment       float64
	)

	for rows.Next() {
		if err := rows.Scan(&id, &name, &lastScrapeTimeHolder, &hourlySentimentHolder); err != nil {
			log.Print(err)
		}

		// MySQL's implementation of Int64 and Float64 make it
		// necessary to use intermediate variables to do our checks.
		// The MySQL driver uses these particular data types as a form
		// of null safety. So we must check if the value the driver finds
		// is null before we can access the actual value.

		lastScrapeTime = 0
		if lastScrapeTimeHolder.Valid {
			lastScrapeTime = lastScrapeTimeHolder.Int64
		}

		hourlySentiment = 0
		if hourlySentimentHolder.Valid {
			hourlySentiment = hourlySentimentHolder.Float64
		}

		tickers.appendTicker(Ticker{
			Name:            name,
			Id:              id,
			LastScrapeTime:  time.Unix(lastScrapeTime, 0),
			HourlySentiment: hourlySentiment,
		})
		log.Printf("%v: %s\n", id, name)
	}
	return tickers, nil
}

const retrieveTickerByIdQuery = `
SELECT ticker_id, name, last_scrape_time FROM tickers ` +
	`WHERE ticker_id=?`

// Retrieves a ticker by ID. Mostly used internally.
func (d DBManager) RetrieveTickerById(tickerId int) (Ticker, error) {
	rows, err := d.db.Query(retrieveTickerByIdQuery, tickerId)
	if err != nil {
		log.Printf("RetrieveTickerById(): Error querying the DB: %v", err)
	}
	defer rows.Close()
	var (
		id             string
		name           string
		lastScrapeTime sql.NullInt64
	)
	strId := strconv.Itoa(tickerId)
	for rows.Next() {
		if err := rows.Scan(&id, &name, &lastScrapeTime); err != nil {
			log.Print(err)
		}
		if strId == id {
			return Ticker{Name: name, Id: tickerId, LastScrapeTime: time.Unix(lastScrapeTime.Int64, 0)}, nil
		}
	}
	return Ticker{Name: "none"}, errors.New("ticker does not exist")
}

type IntervalQuote struct {
	TimeStamp    int64
	CurrentPrice float64
}

const returnSentimentHistoryQuery = `
SELECT time_stamp, hourly_sentiment FROM sentiments ` +
	`WHERE ticker_id=? ORDER BY time_stamp DESC`

// Retrieves the average hourly sentiment over a given time range.
func (d DBManager) ReturnSentimentHistory(id int, fromTime int64) []IntervalQuote {
	rows, err := d.db.Query(returnSentimentHistoryQuery, id)
	if err != nil {
		log.Print("Error returning senitment history: ", err)
	}

	var payload []IntervalQuote
	var s IntervalQuote

	for rows.Next() {
		if rows.Err() != nil {
			log.Print("Found no rows.")
		}
		if err := rows.Scan(&s.TimeStamp, &s.CurrentPrice); err != nil {
			log.Printf("ReturnSentimentHistory(): Error in rows.Scan() for ticker %d: %v", id, err)
		}
		if s.TimeStamp < fromTime {
			break
		}
		payload = append(payload, s)
	}
	return payload
}

const returnAllStatementsQuery = `
SELECT time_stamp, expression, url, polarity, tweet_id ` +
	`FROM statements WHERE ticker_id=? ` +
	`ORDER BY time_stamp DESC`

// Returns all tweets over a given timerange for a ticker specified by ID.
func (d DBManager) ReturnAllStatements(id int, fromTime int64) []twitter.Statement {
	rows, err := d.db.Query(returnAllStatementsQuery, id)
	if err != nil {
		log.Print("Error returning sentiment history: ", err)
		return nil
	}

	var (
		returnPackage []twitter.Statement
		st            twitter.Statement
	)

	for rows.Next() {
		if rows.Err() != nil {
			log.Printf("ReturnAllStatements(): %v", rows.Err())
		}
		if err := rows.Scan(&st.TimeStamp, &st.Expression, &st.PermanentURL, &st.Polarity, &st.ID); err != nil {
			log.Printf("ReturnAllStatements(): Error in rows.Scan() for ticker %d: %v", id, err)
		}
		if st.TimeStamp < fromTime {
			break
		}
		returnPackage = append(returnPackage, st)
	}
	return returnPackage
}

const checkTickerExistsQuery = `
SELECT ticker_id FROM tickers where name=?`

func (d DBManager) CheckTickerExists(ticker string) bool {
	rows, err := d.db.Query(checkTickerExistsQuery, ticker)
	if err != nil {
		log.Printf("Error checking if ticker %s exists: %v", ticker, err)
		return false
	}
	var id int
	for rows.Next() {
		if rows.Err() != nil {
			log.Printf("Error checking if ticker %s exists: %v", ticker, rows.Err())
		}
		if err := rows.Scan(&id); err != nil {
			log.Printf("Error checking if ticker %s exists: %v", ticker, err)
		}
	}
	if id > 0 {
		return true
	}
	return false
}

const deactivateTickerQuery = `
UPDATE tickers SET active=0 WHERE ticker_id=?`

// Sets the ticker active status to 0.
// Prevents the hourly scraping of the ticker.
func (d DBManager) DeactivateTicker(id int) error {
	if _, err := d.db.Exec(deactivateTickerQuery, id); err != nil {
		return err
	}
	return nil
}

func (tickers *TickerSlice) appendTicker(t Ticker) {
	ts := *tickers
	ts = append(ts, t)
	*tickers = ts
}
