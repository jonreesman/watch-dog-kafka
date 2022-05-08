package kafka

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/jonreesman/wdk/db"
	kafka "github.com/segmentio/kafka-go"
)

const (
	ADD_TOPIC    = "add"
	DELETE_TOPIC = "delete"
	SCRAPE_TOPIC = "scrape"
)

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

func SpawnConsumer(kafkaURL string, topic string, groupID string) error {
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
			log.Printf("Failed to read message")
		}
		fmt.Printf("message at topic:%v partition:%v offset:%v	%s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value))
		fmt.Printf("%s", string(m.Value))
		t := ticker{
			Name: string(m.Value),
		}

		if m.Topic == DELETE_TOPIC {
			/*id, err := d.RetrieveTickerIDByName(t.Name)
			if err != nil {
				log.Printf("Consumer: Failed RetrieveTickerIDByName for ticker %s: %v", t.Name, err)
				continue
			}*/
			id, err := strconv.Atoi(t.Name)
			if err != nil {
				log.Printf("Consumer: Failed to convert id from string to int: %v", err)
				continue
			}
			err = d.DeactivateTicker(id)
			if err != nil {
				log.Printf("Consumer: Failed to DeactivateTicker %s with id %d: %v", t.Name, id, err)
			}
			continue
		}

		t.Id, err = d.AddTicker(t.Name)
		if err != nil {
			log.Printf("SpawnWorker(); Could not find/add ticker: %v", err)
		}

		lastScrapeTime, err := d.RetrieveTickerLastScrapeTime(t.Name)
		if err != nil {
			log.Printf("Error retrieiving lastScrapeTime for %s: %v", t.Name, err)
			lastScrapeTime = 0
		}
		t.scrape(lastScrapeTime)
		t.pushToDb(d)
	}

}
