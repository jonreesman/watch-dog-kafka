package consumer

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/jonreesman/watch-dog-kafka/by"
	"github.com/jonreesman/watch-dog-kafka/cleaner"
	"github.com/jonreesman/watch-dog-kafka/db"
	kafka "github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
)

// Defines the Kafka topics for the project
// across the package.
const (
	ADD_TOPIC    = "add"
	DELETE_TOPIC = "delete"
	SCRAPE_TOPIC = "scrape"
)

type ConsumerConfig struct {
	DbManager      db.DBManager
	GrpcServerConn *grpc.ClientConn
	SpamDetector   *by.SpamDetector
	Cleaner        *cleaner.Cleaner
}

// Returns a Kafka reader for a specific topic and group
func getKafkaReader(kafkaURL, topic, groupID string) *kafka.Reader {
	brokers := strings.Split(kafkaURL, ",")
	return kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		GroupID:  groupID,
		Topic:    topic,
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})
}

// Defines our consumer goroutine. Retrieves a Kafka Reader for its topic,
// grabs a connection to the master database, and listes to the topic
// for events. It can handle the logic for deletions, additions, and scrapes.
func SpawnConsumer(ch chan []string, config ConsumerConfig) {
	d := config.DbManager
	for {
		m := <-ch
		/*if err != nil {
			log.Printf("SpawnConsumer(): %v", err)
			sleepTime := 30
			if err.Error() == kafka.BrokerNotAvailable.Error() {
				sleepTime = 120
			}
			if err.Error() == kafka.RebalanceInProgress.Error() {
				sleepTime = 60
			}
			time.Sleep(time.Duration(sleepTime) * time.Second)
			fmt.Printf("Consumer resuming...")
			reader = getKafkaReader(kafkaURL, topic, groupID)
			continue
		}*/
		fmt.Printf("message at topic:%v = %s\n", m[1], m[0])
		if m == nil {
			log.Printf("message value nil. continuing")
			continue
		}
		t := ticker{
			Name: m[0],
			db:   d,
		}

		// If the consumer is a `delete` consumer, it'll exclusively
		// execute this logic. It simply issues a MySQL query to
		// set active to 0 so that no scraping occurs for that ticker.
		if m[1] == DELETE_TOPIC {
			id, err := strconv.Atoi(t.Name)
			if err != nil {
				log.Printf("Consumer: Failed to convert id from string to int: %v", err)
				continue
			}
			if err := d.DeactivateTicker(id); err != nil {
				log.Printf("Consumer: Failed to DeactivateTicker %s with id %d: %v", t.Name, id, err)
			}
			continue
		}

		var err error
		if m[1] == ADD_TOPIC {
			t.Id, err = d.AddTicker(t.Name)
			if err != nil {
				if err.Error() == "ticker active" {
					log.Printf("SpawnWorker(): Ticker already active. Skipping.")
					continue
				}
				log.Printf("SpawnWorker(); Could not add ticker with name %s: %v", t.Name, err)
				continue
			}
		}

		if m[1] == SCRAPE_TOPIC {
			t.Id, err = d.RetrieveTickerIDByName(t.Name)
			if err != nil {
				log.Printf("SpawnWorker(); Could not find ticker with name %s: %v", t.Name, err)
				continue
			}
		}

		t.grpcServerConn = config.GrpcServerConn
		// Grabs the last time the stock was scraped so that we know
		// how far back we must scrape twitter. If none is found (eg. its
		// NULL in the database), we set it to 0 to do an initial scrape.
		lastScrapeTime, err := d.RetrieveTickerLastScrapeTime(t.Name)
		if err != nil {
			log.Printf("Error retrieiving lastScrapeTime for %s: %v", t.Name, err)
			lastScrapeTime = 0
		}
		t.scrape(lastScrapeTime)
		t.spamProcessor(&config)
		if err := t.computeHourlySentiment(); err != nil {
			log.Printf("consumer(): %v\n", err)
			continue
		}
		t.pushToDb()
		t = ticker{}
	}

}
