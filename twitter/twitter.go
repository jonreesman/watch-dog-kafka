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

func TwitterScrape(tickerName string, lastScrapeTime int64) []Statement {
	scraper := twitterscraper.New()
	scraper.SetSearchMode(twitterscraper.SearchLatest)
	scraper.WithReplies(false)
	var tweets []Statement
	for tweet := range scraper.SearchTweets(context.Background(), tickerName+" within_time:1h", 100) {
		if tweet.Error != nil {
			return tweets
		}
		tweet.Text = sanitize(tweet.Text)
		if strings.Contains(tweet.Text, tickerName) {
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
	}
	return tweets
}

func sanitize(s string) string {
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
