package twitter

import (
	"fmt"
	"strconv"
	"testing"
	"time"
)

func TestTwitterScrapeRange(t *testing.T) {
	statements := TwitterScrapeRange(1651691408, 1651777808, "AMD")
	var maxTime int64
	minTime := time.Now().Unix()
	for _, s := range statements {
		if s.TimeStamp < minTime {
			minTime = s.TimeStamp
		}
		if s.TimeStamp > maxTime {
			maxTime = s.TimeStamp
		}
	}
	fmt.Println("Number of tweets: " + strconv.Itoa(len(statements)))
	fmt.Println("minTime: " + time.Unix(minTime, 0).String() + " maxTime: " + time.Unix(maxTime, 0).String())
}
