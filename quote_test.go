package main

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

const charset = `abcdefghijklmnopqrstuvwxyz` +
	`ABCDEFGHIJKLMNOPQRSTUVWXYZ` +
	`1234567890` +
	`!@#$%^&*()-=[]\;',./~`

func randomTickerName() string {
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, rand.Intn(100))
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func TestPriceCheck(t *testing.T) {
	for i := 0; i < 100; i++ {
		priceCheck(randomTickerName())
	}
}

func TestCheckTickerExists(t *testing.T) {
	for i := 0; i < 100; i++ {
		t := randomTickerName()
		if CheckTickerExists(t) {
			fmt.Println(t)
		}
	}
}
