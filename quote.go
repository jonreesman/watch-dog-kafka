package main

import (
	"fmt"
	"log"

	"github.com/piquette/finance-go/quote"
)

func priceCheck(ticker string) float64 {
	q, err := quote.Get(ticker)
	if err != nil {
		log.Printf("Error getting quote")
	}
	return q.RegularMarketPrice
}

func CheckTickerExists(ticker string) bool {
	q, err := quote.Get(ticker)
	fmt.Println(q)
	if err != nil || q == nil {
		return false
	} else {
		return true
	}
}
