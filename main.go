package main

import (
	"fmt"
	"log"
	"os"

	"github.com/jonreesman/wdk/db"
	"github.com/jonreesman/wdk/kafka"
)

func run(kafkaURL string) error {
	d, err := db.NewManager(db.SLAVE)
	if err != nil {
		return err
	}
	for {
		tickers, _ := d.ReturnActiveTickers()
		for _, ticker := range tickers {
			kafka.ProducerHandler(nil, kafkaURL, kafka.SCRAPE_TOPIC, ticker.Name)
		}
	}
}

func main() {
	fmt.Println("Hello")
	kafkaURL := os.Getenv("kafkaURL")
	groupID := os.Getenv("groupID")

	for i := 0; i < 10; i++ {
		go kafka.SpawnConsumer(kafkaURL, kafka.ADD_TOPIC, groupID)
		go kafka.SpawnConsumer(kafkaURL, kafka.DELETE_TOPIC, groupID)
		go kafka.SpawnConsumer(kafkaURL, kafka.SCRAPE_TOPIC, groupID)
	}

	s, err := NewServer(kafkaURL)
	if err != nil {
		log.Fatal(err)
	}

	if err := s.startServer(); err != nil {
		log.Fatal(err)
	}
	if err := run(kafkaURL); err != nil {
		log.Fatal(err)
	}
}
