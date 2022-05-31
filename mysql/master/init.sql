create user 'slave_user'@'%' identified by 'password';
grant replication slave on *.* to 'slave_user'@'%' with grant option;
flush privileges;

use app;
CREATE TABLE IF NOT EXISTS tickers(ticker_id SERIAL PRIMARY KEY, name VARCHAR(255), active INT, last_scrape_time BIGINT);
ALTER TABLE tickers ADD CONSTRAINT ticker_Unique UNIQUE(name);

CREATE TABLE IF NOT EXISTS statements(tweet_id BIGINT UNSIGNED PRIMARY KEY, ticker_id BIGINT UNSIGNED, expression VARCHAR(500), url VARCHAR(255), time_stamp BIGINT, polarity FLOAT, FOREIGN KEY (ticker_id) REFERENCES tickers(ticker_id) ON DELETE CASCADE);
ALTER TABLE statements ADD CONSTRAINT url_Unique UNIQUE(url);

CREATE TABLE IF NOT EXISTS sentiments(sentiment_id SERIAL PRIMARY KEY, time_stamp BIGINT, ticker_id BIGINT UNSIGNED, hourly_sentiment FLOAT, FOREIGN KEY (ticker_id) REFERENCES tickers(ticker_id) ON DELETE CASCADE);