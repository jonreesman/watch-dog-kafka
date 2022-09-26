package db

import (
	"context"
	"database/sql"
	"log"

	"github.com/jonreesman/watch-dog-kafka/twitter"
)

const addStatementQuery = `
INSERT INTO statements(ticker_id, expression, time_stamp, polarity, url, tweet_id, likes, replies, retweets, spam) ` +
	`VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// Adds a single tweet to the statement table of the database.
func (dbManager DBManager) AddStatement(tickerId int, expression string, timeStamp int64, polarity float64, url string, tweet_id uint64, likes, replies, retweets int) {
	_, err := dbManager.db.Exec(addStatementQuery,
		tickerId,
		expression,
		timeStamp,
		float32(polarity),
		url,
		tweet_id,
		likes,
		replies,
		retweets,
	)
	if err != nil {
		log.Print("Error in addStatement", err)
	}
}

// Adds a single tweet to the statement table of the database.
func (dbManager DBManager) AddStatements(t *sql.Tx, tickerId int, expression string, timeStamp int64, polarity float64, url string, tweet_id uint64, likes, replies, retweets int, spam bool) {
	_, err := t.Exec(addStatementQuery,
		tickerId,
		expression,
		timeStamp,
		float32(polarity),
		url,
		tweet_id,
		likes,
		replies,
		retweets,
		spam,
	)
	if err != nil {
		log.Print("Error in addStatements(): ", err)
	}
}

func (dbManager DBManager) BeginTx() *sql.Tx {
	t, err := dbManager.db.BeginTx(context.Background(), nil)
	if err != nil {
		log.Printf("Error beginning transaction: %v", err)
	}
	return t
}

const returnAllStatementsQuery = `
SELECT time_stamp, expression, url, polarity, tweet_id, likes, replies, retweets ` +
	`FROM statements WHERE ticker_id=? ` +
	`ORDER BY time_stamp DESC`

// Returns all tweets over a given timerange for a ticker specified by ID.
func (dbManager DBManager) ReturnAllStatements(id int, fromTime int64) []twitter.Statement {
	rows, err := dbManager.db.Query(returnAllStatementsQuery, id)
	if err != nil {
		log.Print("Error returning sentiment history: ", err)
		return nil
	}

	defer rows.Close()
	var (
		returnPackage []twitter.Statement
		statement     twitter.Statement
		likes         sql.NullInt64
		replies       sql.NullInt64
		retweets      sql.NullInt64
	)

	for rows.Next() {
		if rows.Err() != nil {
			log.Printf("ReturnAllStatements(): %v", rows.Err())
		}
		if err := rows.Scan(&statement.TimeStamp, &statement.Expression, &statement.PermanentURL, &statement.Polarity, &statement.ID, &likes, &replies, &retweets); err != nil {
			log.Printf("ReturnAllStatements(): Error in rows.Scan() for ticker %d: %v", id, err)
		}
		if statement.TimeStamp < fromTime {
			break
		}
		if likes.Valid {
			statement.Likes = int(likes.Int64)
		}
		if replies.Valid {
			statement.Replies = int(replies.Int64)
		}
		if retweets.Valid {
			statement.Retweets = int(retweets.Int64)
		}
		returnPackage = append(returnPackage, statement)
	}
	return returnPackage
}
