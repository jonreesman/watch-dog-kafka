package db

import (
	"log"
	"sync"
)

type IntervalQuote struct {
	TimeStamp    int64
	CurrentPrice float64
}

const addSentimentQuery = `
INSERT INTO sentiments(time_stamp, ticker_id, hourly_sentiment) ` +
	`VALUES (?, ?, ?)`

// Adds an average hourly sentiment to the database.
// less concerned with any errors, it does not disrupt the application,
// but does generate an error message for diagnosing issues.
func (dbManager DBManager) AddSentiment(wg *sync.WaitGroup, timeStamp int64, tickerId int, hourlySentiment float64) {
	defer wg.Done()
	_, err := dbManager.db.Exec(addSentimentQuery,
		timeStamp,
		tickerId,
		float32(hourlySentiment),
	)
	if err != nil {
		log.Print("Error in addSentiment()", err, hourlySentiment)
		log.Print("id is ", tickerId)
	}
}

const returnSentimentHistoryQuery = `
SELECT time_stamp, hourly_sentiment FROM sentiments ` +
	`WHERE ticker_id=? ORDER BY time_stamp DESC`

// Retrieves the average hourly sentiment over a given time range.
func (dbManager DBManager) ReturnSentimentHistory(id int, fromTime int64) []IntervalQuote {
	rows, err := dbManager.db.Query(returnSentimentHistoryQuery, id)
	if err != nil {
		log.Print("Error returning senitment history: ", err)
	}
	defer rows.Close()

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
