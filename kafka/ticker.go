package kafka

import (
	"context"

	"fmt"
	"log"
	"sync"
	"time"

	"github.com/jonreesman/wdk/db"
	"github.com/jonreesman/wdk/pb"
	"github.com/jonreesman/wdk/twitter"
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

// DEPRECATED. Wipes our ticker object between
// scrapes. DELETE.
/*func (t *ticker) hourlyWipe() {
	t.numTweets = 0
	t.Tweets = nil
}*/

// Handles pushing all relevant ticker information to the database concurrently.
// It will push all tweets and hourly sentiments to the DB and update the lastScrapeTime.
func (t ticker) pushToDb(d db.DBManager) {
	var wg sync.WaitGroup
	wg.Add(1)
	go d.AddSentiment(&wg, t.LastScrapeTime.Unix(), t.Id, t.HourlySentiment)
	wg.Add(1)
	go d.UpdateTicker(&wg, t.Id, t.LastScrapeTime)
	for _, tw := range t.Tweets {
		fmt.Println("added statement to DB for:", tw.Subject)
		d.AddStatement(t.Id, tw.Expression, tw.TimeStamp, tw.Polarity, tw.PermanentURL, tw.ID)
	}
	wg.Wait()
}

// Given lastScrapeTime, will scrape Twitter for all tweets
// back to that time, then compute the hourly sentiment.
func (t *ticker) scrape(lastScrapeTime int64) {
	t.Tweets = append(t.Tweets, twitter.TwitterScrape(t.Name, lastScrapeTime)...)
	t.numTweets = len(t.Tweets)
	t.LastScrapeTime = time.Now()
	t.computeHourlySentiment()
}

// Utilizes GRPC to communicate with our Python ancillary that performs
// sentiment analysis on the tweets for a given ticker.
func (t *ticker) computeHourlySentiment() {
	var total float64
	addr := "localhost:9999"
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Printf("computeHourlySentiment(): Failed to dial GRPC.")
		return
	}
	defer conn.Close()
	client := pb.NewSentimentClient(conn)
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
}
