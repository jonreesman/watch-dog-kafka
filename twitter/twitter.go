package twitter

import (
	"context"
	"log"
	"regexp"
	"strconv"
	"strings"

	"github.com/forPelevin/gomoji"
	twitterscraper "github.com/n0madic/twitter-scraper"
)

// This struct defines how a tweet is treated with respect to
// the business logic as well as within the DB. It is left
// source agnostic to allow for future expansion to other
// scraping sources. This struct will likely move once more are added.
type Statement struct {
	Expression   string
	Subject      string
	Source       string
	TimeStamp    int64
	Polarity     float64
	URLs         []string
	PermanentURL string
	ID           uint64
}

// Returns most tweets for a given stock or ticker name with a given
// fromTime. This fromTime is the last time Twitter was scraped for the
// stock or crypto.
func TwitterScrape(tickerName string, lastScrapeTime int64) []Statement {
	scraper := twitterscraper.New()

	scraper.SetSearchMode(twitterscraper.SearchTop)
	scraper.WithReplies(false)
	var tweets []Statement

	// Since we scrape hourly, we are only concerned
	// with all the tweets within the past hour.
	for tweet := range scraper.SearchTweets(context.Background(),
		tickerName+" within_time:1h", 100) {
		if tweet.Error != nil {
			return tweets
		}

		// Removes certain characters and replaces Emojis.
		tweet.Text = SanitizeTweet(tweet.Text)

		// Secondary check to ensure the tweet is actually
		// about our stock of choice.
		if !strings.Contains(tweet.Text, tickerName) {
			continue
		}

		if tweet.Timestamp < lastScrapeTime {
			break
		}

		id, err := strconv.ParseUint(tweet.ID, 10, 64)
		if err != nil {
			log.Printf("twitterScrape(): Error extracting tweet id")
		}
		s := Statement{
			Expression:   tweet.Text,
			Subject:      tickerName,
			Source:       "Twitter",
			TimeStamp:    tweet.Timestamp,
			Polarity:     0,
			URLs:         tweet.URLs,
			PermanentURL: tweet.PermanentURL,
			ID:           id,
		}
		tweets = append(tweets, s)

	}
	return tweets
}

func TwitterScrapeRange(fromTime, toTime int64, tickerName string) []Statement {
	scraper := twitterscraper.New()

	scraper.SetSearchMode(twitterscraper.SearchTop)
	scraper.WithReplies(false)
	var tweets []Statement

	// Since we scrape hourly, we are only concerned
	// with all the tweets within the past hour.
	log.Print(tickerName + " since_time:" + strconv.FormatInt(fromTime, 10) + " until_time:" + strconv.FormatInt(toTime, 10))
	for tweet := range scraper.SearchTweets(context.Background(),
		tickerName+" since_time:"+strconv.FormatInt(fromTime, 10)+" until_time:"+strconv.FormatInt(toTime, 10), 100) {
		if tweet.Error != nil {
			log.Printf("TwitterScrapeRange(): Error in SearchTweets() %v", tweet.Error)
			return tweets
		}

		// Removes certain characters and replaces Emojis.
		tweet.Text = SanitizeTweet(tweet.Text)

		// Secondary check to ensure the tweet is actually
		// about our stock of choice.
		if !strings.Contains(tweet.Text, tickerName) {
			continue
		}
		id, err := strconv.ParseUint(tweet.ID, 10, 64)
		if err != nil {
			log.Printf("twitterScrape(): Error extracting tweet id")
		}
		s := Statement{
			Expression:   tweet.Text,
			Subject:      tickerName,
			Source:       "Twitter",
			TimeStamp:    tweet.Timestamp,
			Polarity:     0,
			URLs:         tweet.URLs,
			PermanentURL: tweet.PermanentURL,
			ID:           id,
		}
		tweets = append(tweets, s)

	}
	return tweets
}

func SanitizeTweet(s string) string {
	emojis := gomoji.FindAll(s)
	regex := regexp.MustCompile("[[:^ascii:]]")

	/* Regex is used here to remove emojis as well.
	 * The accompanying gomoji package is efficient
	 * for finding emojis in the string but not for
	 * removing them.
	 */
	//s = gomoji.RemoveEmojis(s)
	s = regex.ReplaceAllLiteralString(s, "")
	if len(s) == 0 {
		return ""
	}
	for _, emoji := range emojis {
		s += " " + emoji.Slug + " "
	}
	s = strings.ReplaceAll(s, "\"", "'")
	s = strings.ReplaceAll(s, ";", "semi-colon")
	return s
}
