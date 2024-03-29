package db

import "log"

func (dbManager DBManager) dropTable(s string) {
	//REMOVE ONCE DONE DEBUGGING
	_, err := dbManager.db.Exec("SET FOREIGN_KEY_CHECKS = 0")
	if err != nil {
		log.Fatal(err)
	}
	_, err = dbManager.db.Exec("DROP TABLE IF EXISTS " + s)
	if err != nil {
		log.Fatal(err)
	}
	_, err = dbManager.db.Exec("SET FOREIGN_KEY_CHECKS = 1")
	if err != nil {
		log.Fatal(err)
	}
}

func (dbManager DBManager) createTickerTable() {
	_, err := dbManager.db.Exec("CREATE TABLE IF NOT EXISTS tickers(ticker_id SERIAL PRIMARY KEY, name VARCHAR(255), active INT, last_scrape_time BIGINT)")
	if err != nil {
		log.Fatal(err)
	}
	_, err = dbManager.db.Exec("ALTER TABLE tickers ADD CONSTRAINT ticker_Unique UNIQUE(name)")
	if err != nil {
		log.Print(err)
	}
}

func (dbManager DBManager) createStatementTable() {
	_, err := dbManager.db.Exec("CREATE TABLE IF NOT EXISTS statements(tweet_id BIGINT UNSIGNED PRIMARY KEY, ticker_id BIGINT UNSIGNED, expression VARCHAR(500), url VARCHAR(255), time_stamp BIGINT, polarity FLOAT, FOREIGN KEY (ticker_id) REFERENCES tickers(ticker_id) ON DELETE CASCADE)")
	if err != nil {
		log.Fatal(err)
	}
	_, err = dbManager.db.Exec("ALTER TABLE statements ADD CONSTRAINT url_Unique UNIQUE(url)")
	if err != nil {
		log.Print(err)
	}
}

func (dbManager DBManager) createSentimentTable() {
	_, err := dbManager.db.Exec("CREATE TABLE IF NOT EXISTS sentiments(sentiment_id SERIAL PRIMARY KEY, time_stamp BIGINT, ticker_id BIGINT UNSIGNED, hourly_sentiment FLOAT, FOREIGN KEY (ticker_id) REFERENCES tickers(ticker_id) ON DELETE CASCADE)")
	if err != nil {
		log.Fatal(err)
	}
}
