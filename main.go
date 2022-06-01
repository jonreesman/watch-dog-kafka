package main

import (
	"log"
	"os"
	"time"

	"github.com/jonreesman/watch-dog-kafka/db"
	"github.com/jonreesman/watch-dog-kafka/kafka"
)

const (
	KAFKA_CONSUMERS_EA = 10
	SLEEP_INTERVAL     = 3600 * time.Second
)

// Run is our central loop that signals hourly to scrape for
// our active stock tickers and cryptocurrencies. Every hour,
// it creates a message on our Kafka `scrape` topic, which
// signals to our consumers to scrape for that stock/crypto.
func run(kafkaURL string) error {
	// Grabs a database manager for our slave/replica database.
	// Only our Kafka consumers have access to the main/master DB.
	d, err := db.NewManager(db.SLAVE)
	if err != nil {
		return err
	}

	// Grabs all active stock tickers every hour, and generates
	// a scrape message on the `scrape` Kafka topic.
	for {
		tickers, _ := d.ReturnActiveTickers()
		for _, ticker := range tickers {
			log.Printf("Ticker %s", ticker.Name)
			kafka.ProducerHandler(nil, kafkaURL, kafka.SCRAPE_TOPIC, ticker.Name)
		}
		time.Sleep(SLEEP_INTERVAL)
	}
}

func main() {
	kafkaURL := os.Getenv("kafkaURL")
	groupID := os.Getenv("groupID")
	grpcHost := os.Getenv("GRPC_HOST")

	// Utilizes goroutines to create concurrent Kafka Consumers.
	for i := 0; i < KAFKA_CONSUMERS_EA; i++ {
		go kafka.SpawnConsumer(kafkaURL, kafka.ADD_TOPIC, groupID)
		go kafka.SpawnConsumer(kafkaURL, kafka.DELETE_TOPIC, groupID)
		go kafka.SpawnConsumer(kafkaURL, kafka.SCRAPE_TOPIC, groupID)
	}

	// Grabs an instance of our Gin server, passing the kafkaURL.
	// Gin server requires the KafkaURL so that it can create
	// its own Kafka producers.
	s, err := NewServer(kafkaURL, grpcHost)
	if err != nil {
		log.Fatal(err)
	}

	// Fails and aborts if the Gin server fails to launch,
	// since there is no reason to run without the API.
	go s.startServer()

	// Launches the hourly loop that results in a regular
	// scraping for each stock ticker/crypto. If this fails,
	// we will also abort.
	if err := run(kafkaURL); err != nil {
		log.Fatal(err)
	}
}
