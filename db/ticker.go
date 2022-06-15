package db

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strconv"
	"sync"
	"time"
)

const activateTickerQuery = `
UPDATE tickers SET active=1 WHERE ticker_id=?`
const addTickerQuery = `
INSERT INTO tickers(name, active, last_scrape_time) ` +
	`VALUES (?,?,?)`

// Checks if a ticker already exists, then attempts to add
// the ticker to the database if not. Returns the id assigned
// to the ticker if successful, else returns 0 and an error.
// Id 0 is reserved by the program for error purposes.
func (dbManager DBManager) AddTicker(name string) (int, error) {
	if name == "" {
		return 0, errors.New("ticker name is blank")
	}
	if t, err := dbManager.RetrieveTickerByName(name); err == nil {
		if t.Active == 1 {
			return t.Id, errors.New("ticker active")
		}
		if _, err := dbManager.db.Exec(activateTickerQuery, t.Id); err != nil {
			return 0, err
		}
		return t.Id, nil
	}

	dbQuery, err := dbManager.db.Prepare(addTickerQuery)
	defer dbQuery.Close()
	if err != nil {
		log.Print("Error in AddTicker()", err)
		return 0, err
	}
	if row, err := dbQuery.Query(name, 1, nil); err != nil {
		return 0, err
	} else {
		row.Close()
	}

	id, err := dbManager.RetrieveTickerIDByName(name)
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
func (dbManager DBManager) UpdateTicker(wg *sync.WaitGroup, id int, timeStamp time.Time) error {
	defer wg.Done()
	if _, err := dbManager.db.Exec(updateTickerQuery, timeStamp.Unix(), id); err != nil {
		return err
	}
	return nil
}

const retrieveTickerByNameQuery = `
SELECT ticker_id, name, last_scrape_time, active FROM tickers ` +
	`WHERE name=?`

// Allows the user to retrieve a ticker entry from the database
// with only the ticker name. Ticker name is assumed to be
// unique, as that is true for the NASDAQ.
func (dbManager DBManager) RetrieveTickerByName(tickerName string) (Ticker, error) {
	rows, err := dbManager.db.Query(retrieveTickerByNameQuery, tickerName)
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
func (dbManager DBManager) RetrieveTickerIDByName(tickerName string) (int, error) {
	rows, err := dbManager.db.Query(retrieveTickerByNameQuery, tickerName)
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
func (dbManager DBManager) RetrieveTickerLastScrapeTime(tickerName string) (int64, error) {
	rows, err := dbManager.db.Query(retrieveTickerByNameQuery, tickerName)
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
func (dbManager DBManager) ReturnActiveTickers(ctx context.Context) (tickers TickerSlice, err error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rows, err := dbManager.db.QueryContext(ctx, activeTickerQuery)
	if err != nil {
		log.Printf("Database Error in ReturnActiveTickers(): %v", err)
		return nil, err
	}
	log.Printf("ReturnActiveTickers(): Query complete.")
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
	}
	return tickers, nil
}

const retrieveTickerByIdQuery = `
SELECT ticker_id, name, last_scrape_time FROM tickers ` +
	`WHERE ticker_id=?`

// Retrieves a ticker by ID. Mostly used internally.
func (dbManager DBManager) RetrieveTickerById(tickerId int) (Ticker, error) {
	rows, err := dbManager.db.Query(retrieveTickerByIdQuery, tickerId)
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

const checkTickerExistsQuery = `
SELECT ticker_id FROM tickers where name=?`

func (dbManager DBManager) CheckTickerExists(ticker string) bool {
	rows, err := dbManager.db.Query(checkTickerExistsQuery, ticker)
	if err != nil {
		log.Printf("Error checking if ticker %s exists: %v", ticker, err)
		return false
	}
	defer rows.Close()
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

const retrieveOldestTweetTimestampQuery = `
SELECT time_stamp FROM statements WHERE ticker_id=? ORDER BY time_stamp ASC LIMIT 1
`

// Retrieves the last scrape time for a ticker.
func (dbManager DBManager) RetrieveOldestTweetTimestamp(tickerName string) (int64, error) {
	rows, err := dbManager.db.Query(retrieveOldestTweetTimestampQuery, tickerName)
	if err != nil {
		log.Print(err)
	}
	defer rows.Close()
	var (
		oldestTweetTimestamp sql.NullInt64
	)
	for rows.Next() {
		if err := rows.Scan(&oldestTweetTimestamp); err != nil {
			log.Print(err)
			return 0, errors.New("Error in retrieveOldestTweetTimestamp():" + err.Error())
		}
	}
	if oldestTweetTimestamp.Valid {
		return oldestTweetTimestamp.Int64, nil
	} else {
		return 0, errors.New("Error in retrieveOldestTweetTimestamp: int64 value retrieved not valid.")
	}
}

const deactivateTickerQuery = `
UPDATE tickers SET active=0 WHERE ticker_id=?`

// Sets the ticker active status to 0.
// Prevents the hourly scraping of the ticker.
func (dbManager DBManager) DeactivateTicker(id int) error {
	if _, err := dbManager.db.Exec(deactivateTickerQuery, id); err != nil {
		return err
	}
	return nil
}

func (tickers *TickerSlice) appendTicker(t Ticker) {
	ts := *tickers
	ts = append(ts, t)
	*tickers = ts
}
