package db

import (
	"log"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

type TestStatement struct {
	Expression   string
	TimeStamp    int64
	Polarity     float64
	PermanentURL string
	ID           uint64
}

func randomStatement() TestStatement {
	return TestStatement{
		Expression:   strconv.FormatUint(rand.Uint64(), 10),
		ID:           rand.Uint64(),
		Polarity:     float64(rand.Int63()),
		PermanentURL: strconv.FormatUint(rand.Uint64(), 10),
		TimeStamp:    rand.Int63(),
	}
}
func TestAddStatements(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	d, err := NewManager(MASTER)
	if err != nil {
		log.Fatal(err)
	}

	id, err := d.AddTicker(strconv.FormatUint(rand.Uint64(), 10))
	if err != nil {
		log.Fatal(err)
	}
	tx := d.BeginTx()
	for i := 0; i < 500; i++ {
		s := randomStatement()
		d.AddStatements(tx, id, s.Expression, s.TimeStamp, s.Polarity, s.PermanentURL, s.ID)
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
}
