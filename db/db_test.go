package db

import (
	"fmt"
	"log"
	"math/rand"
	"os"
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
	Likes        int
	Replies      int
	Retweets     int
}

func randomStatement() TestStatement {
	return TestStatement{
		Expression:   strconv.FormatUint(rand.Uint64(), 10),
		ID:           rand.Uint64(),
		Polarity:     float64(rand.Int63()),
		PermanentURL: strconv.FormatUint(rand.Uint64(), 10),
		TimeStamp:    rand.Int63(),
		Likes:        rand.Intn(1000),
		Replies:      rand.Intn(1000),
		Retweets:     rand.Intn(1000),
	}
}
func TestAddStatements(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dbUser := os.Getenv("DB_USER")
	dbPwd := os.Getenv("DB_PWD")
	dbName := os.Getenv("DB_NAME")
	dbMasterURL := os.Getenv("DB_MASTER")

	d, err := NewManager(dbUser, dbPwd, dbName, dbMasterURL)
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
		d.AddStatements(tx, id, s.Expression, s.TimeStamp, s.Polarity, s.PermanentURL, s.ID, s.Likes, s.Replies, s.Retweets, false)
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	s := d.ReturnAllStatements(id, 0)
	for i, st := range s {
		fmt.Printf("%d: Likes: %d  Replies: %d  Retweets: %d", i, st.Likes, st.Replies, st.Retweets)
	}
}
