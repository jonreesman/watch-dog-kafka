package kafka

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jonreesman/watch-dog-kafka/db"
	kafka "github.com/segmentio/kafka-go"
)

// Defines the Kafka topics for the project
// across the package.
const (
	ADD_TOPIC    = "add"
	DELETE_TOPIC = "delete"
	SCRAPE_TOPIC = "scrape"
)

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
func SpawnConsumer(kafkaURL string, topic string, groupID string) error {
	log.Printf("Starting consumer with kafkaURL %s", kafkaURL)
	reader := getKafkaReader(kafkaURL, topic, groupID)
	d, err := db.NewManager(db.MASTER)
	if err != nil {
		log.Printf("Error establishing DB connection: %v", err)
	}
	defer reader.Close()
	defer d.Close()

	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("Consumer failed to read message with error: %v", err)
			if err.Error() == kafka.RebalanceInProgress.Error() {
				time.Sleep(3000 * time.Second)
				//Reconnect
				reader = getKafkaReader(kafkaURL, topic, groupID)
			}
			continue
		}
		fmt.Printf("message at topic:%v partition:%v offset:%v	%s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
		fmt.Printf("%s", string(m.Value))
		if m.Value == nil {
			log.Printf("message value nil. continuing")
			continue
		}
		t := ticker{
			Name: string(m.Value),
		}

		// If the consumer is a `delete` consumer, it'll exclusively
		// execute this logic. It simply issues a MySQL query to
		// set active to 0 so that no scraping occurs for that ticker.
		if m.Topic == DELETE_TOPIC {
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

		if m.Topic == ADD_TOPIC {
			t.Id, err = d.AddTicker(t.Name)
			if err != nil {
				log.Printf("SpawnWorker(); Could not add ticker with name %s: %v", t.Name, err)
			}
		}

		if m.Topic == SCRAPE_TOPIC {
			t.Id, err = d.RetrieveTickerIDByName(t.Name)
			if err != nil {
				log.Printf("SpawnWorker(); Could not find ticker with name %s: %v", t.Name, err)
			}
		}

		// Grabs the last time the stock was scraped so that we know
		// how far back we must scrape twitter. If none is found (eg. its
		// NULL in the database), we set it to 0 to do an initial scrape.
		lastScrapeTime, err := d.RetrieveTickerLastScrapeTime(t.Name)
		if err != nil {
			log.Printf("Error retrieiving lastScrapeTime for %s: %v", t.Name, err)
			lastScrapeTime = 0
		}
		t.scrape(lastScrapeTime)
		t.pushToDb(d)
	}

}
