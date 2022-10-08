package consumer

import (
	"context"

	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jonreesman/watch-dog-kafka/db"
	"github.com/jonreesman/watch-dog-kafka/pb"
	"github.com/jonreesman/watch-dog-kafka/twitter"
	"google.golang.org/grpc"
)

type tickerSlice []ticker

//Defines a ticker object packaged for pushing to a database.
type ticker struct {
	Name            string
	LastScrapeTime  time.Time
	numTweets       int
	Tweets          []twitter.Statement
	HourlySentiment float64
	Id              int
	Active          int
	grpcServerConn  *grpc.ClientConn
	db              db.DBManager
}

// Defines a statement object. Primarily refers to a tweet,
// but it leaves room for the addition of other sources.

// Defines a stock quote and the time that quote was taken.
// Also is frequently used to sentiments over time since
// the variable types are identical.
type intervalQuote struct {
	TimeStamp    int64
	CurrentPrice float64
}

// Handles pushing all relevant ticker information to the database concurrently.
// It will push all tweets and hourly sentiments to the DB and update the lastScrapeTime.
func (t ticker) pushToDb() {
	var wg sync.WaitGroup
	db := t.db
	wg.Add(1)
	go db.AddSentiment(&wg, t.LastScrapeTime.Unix(), t.Id, t.HourlySentiment)
	wg.Add(1)
	go db.UpdateTicker(&wg, t.Id, t.LastScrapeTime)
	tx := db.BeginTx()
	for _, tw := range t.Tweets {
		fmt.Println("added statement to DB for:", tw.Subject)
		db.AddStatements(tx, t.Id, tw.Expression, tw.TimeStamp, tw.Polarity, tw.PermanentURL, tw.ID, tw.Likes, tw.Replies, tw.Retweets, tw.Spam)
	}
	if err := tx.Commit(); err != nil {
		log.Printf("Error pushing %s tweets to DB: %v", t.Name, err)
	}
	wg.Wait()
	t.Tweets = nil
}

// Given lastScrapeTime, will scrape Twitter for all tweets
// back to that time, then compute the hourly sentiment.
func (t *ticker) scrape(lastScrapeTime int64) {
	t.Tweets = twitter.TwitterScrape(t.Name, lastScrapeTime)
	t.numTweets = len(t.Tweets)
	t.LastScrapeTime = time.Now()
}

func (t *ticker) spamProcessor(config *ConsumerConfig) {
	for _, tweet := range t.Tweets {
		cleaned := config.Cleaner.CleanText(tweet.Expression)
		score, _, _ := config.SpamDetector.Classifier.ProbScores(cleaned)
		if score[0] > score[1] {
			tweet.Spam = false
		} else {
			tweet.Spam = true
		}
	}
}

// Utilizes GRPC to communicate with our Python ancillary that performs
// sentiment analysis on the tweets for a given ticker.
func (t *ticker) computeHourlySentiment() {
	var total float64
	client := pb.NewSentimentClient(t.grpcServerConn)
	for i, s := range t.Tweets {
		request := pb.SentimentRequest{
			Tweet: s.Expression,
		}
		response, err := client.Detect(context.Background(), &request)
		if err != nil {
			log.Printf("GRPC SentimentRequest: %v", err)
		}
		t.Tweets[i].Polarity = float64(response.Polarity)
		total += float64(response.Polarity)
	}
	t.HourlySentiment = total / float64(t.numTweets)
	if total == 0 || t.numTweets == 0 {
		t.HourlySentiment = 0
	}
}
