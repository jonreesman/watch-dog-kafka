package main

import (
	"fmt"
	"log"
	"regexp"

	"github.com/piquette/finance-go/quote"
)

/*
	Grabs a quick price check using the piquette finance
	package for Go. It is important to note that this package
	differs from the one used to grab the price history. This
	package is much more limited, and thus does not effectively
	pull stock quote history, but it has less overhead than the Python
	module used for quote history.
*/
func priceCheck(ticker string) float64 {
	q, err := quote.Get(ticker)
	if err != nil {
		log.Printf("Error getting quote")
		return 0
	}
	if q == nil {
		return 0
	}
	return q.RegularMarketPrice
}

// Uses the piquette Finance package to validate against known
// exchanges that the specific stock ticker exists.
func CheckTickerExists(ticker string) bool {
	q, err := quote.Get(ticker)
	fmt.Println(q)
	if err != nil || q == nil {
		return false
	} else {
		return true
	}
}

func SanitizeTicker(s string) string {
	regex := regexp.MustCompile("/[A-Za-z]")
	s = regex.ReplaceAllLiteralString(s, "")
	if len(s) == 0 {
		return ""
	}
	return s
}
