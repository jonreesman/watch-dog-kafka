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
	"github.com/jonreesman/wdk/twitter"
)

type DBManager struct {
	db     *sql.DB
	dbName string
	dbUser string
	dbPwd  string
	dbPort string
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

func NewManager(t Type) (DBManager, error) {
	var d DBManager
	var err error
	d.dbUser = os.Getenv("DB_USER")
	d.dbPwd = os.Getenv("DB_PWD")
	d.dbName = os.Getenv("DB_NAME")

	if t == MASTER {
		d.dbPort = os.Getenv("DB_MASTER_PORT")
	} else {
		d.dbPort = os.Getenv("DB_SLAVE_PORT")
	}

	d.URI = fmt.Sprintf("%s:%s@tcp(127.0.0.1:%s)/%s", d.dbUser, d.dbPwd, d.dbPort, d.dbName)
	d.db, err = sql.Open("mysql", d.URI)
	if err != nil {
		log.Printf("Failed to open connection in DB NewManager(): %v", err)
		return DBManager{}, err
	}

	err = d.db.Ping()
	if err != nil {
		log.Printf("Failed to Ping DB in NewManager(): %v", err)
		return DBManager{}, err
	}
	_, err = d.db.Exec(fmt.Sprintf("USE %s", d.dbName))
	if err != nil {
		log.Printf("Failed to `USE %s` in NewManager(): %v", d.dbName, err)
		return DBManager{}, err
	}
	fmt.Println("Connection established")
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

func (d DBManager) AddTicker(name string) (int, error) {
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
	_, err = dbQuery.Query(name, 1, nil)
	if err != nil {
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
		if name == tickerName {
			t := Ticker{
				Id:             id,
				Name:           name,
				LastScrapeTime: time.Unix(lastScrapeTime.Int64, 0),
				Active:         active,
			}
			return t, nil
		}
	}
	return Ticker{Id: 0, Name: ""}, errors.New("ticker does not exist with that ID")
}

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
		err := rows.Scan(&id, &name, &lastScrapeTime, &active)
		if err != nil {
			log.Print(err)
		}
		if name == tickerName {
			return id, nil
		}
	}
	return 0, errors.New("ticker does not exist with that ID")
}

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
		err := rows.Scan(&id, &name, &lastScrapeTime, &active)
		if err != nil {
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
		err := rows.Scan(&id, &name, &lastScrapeTimeHolder, &hourlySentimentHolder)
		if err != nil {
			log.Print(err)
		}
		if lastScrapeTimeHolder.Valid {
			lastScrapeTime = lastScrapeTimeHolder.Int64
		} else {
			lastScrapeTime = 0
		}
		if hourlySentimentHolder.Valid {
			hourlySentiment = hourlySentimentHolder.Float64
		} else {
			hourlySentiment = 0
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

func (tickers *TickerSlice) appendTicker(t Ticker) {
	ts := *tickers
	ts = append(ts, t)
	*tickers = ts
}

const retrieveTickerByIdQuery = `
SELECT ticker_id, name, last_scrape_time FROM tickers ` +
	`WHERE ticker_id=?`

func (d DBManager) RetrieveTickerById(tickerId int) (Ticker, error) {
	rows, err := d.db.Query(retrieveTickerByIdQuery, tickerId)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	var (
		id             string
		name           string
		lastScrapeTime sql.NullInt64
	)
	strId := strconv.Itoa(tickerId)
	for rows.Next() {
		err := rows.Scan(&id, &name, &lastScrapeTime)
		if err != nil {
			log.Fatal(err)
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
		err := rows.Scan(&s.TimeStamp, &s.CurrentPrice)
		if s.TimeStamp < fromTime {
			break
		}
		payload = append(payload, s)
		if err != nil {
			log.Print("Error in row scan")
		}
	}
	return payload
}

const returnAllStatementsQuery = `
SELECT time_stamp, expression, url, polarity, tweet_id ` +
	`FROM statements WHERE ticker_id=? ` +
	`ORDER BY time_stamp DESC`

func (d DBManager) ReturnAllStatements(id int, fromTime int64) []twitter.Statement {
	rows, err := d.db.Query(returnAllStatementsQuery, id)
	if err != nil {
		log.Print("Error returning senitment history: ", err)
	}

	var (
		returnPackage []twitter.Statement
		st            twitter.Statement
	)

	for rows.Next() {
		if rows.Err() != nil {
			log.Print("Found no rows.")
		}
		err := rows.Scan(&st.TimeStamp, &st.Expression, &st.PermanentURL, &st.Polarity, &st.ID)
		if st.TimeStamp < fromTime {
			break
		}
		returnPackage = append(returnPackage, st)
		if err != nil {
			log.Print("Error in row scan")
		}
	}
	return returnPackage
}

const deactivateTickerQuery = `
UPDATE tickers SET active=0 WHERE ticker_id=?`

func (d DBManager) DeactivateTicker(id int) error {
	if _, err := d.db.Exec(deactivateTickerQuery, id); err != nil {
		return err
	}
	return nil
}
