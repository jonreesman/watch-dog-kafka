package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jonreesman/watch-dog-kafka/db"
	"github.com/jonreesman/watch-dog-kafka/kafka"
)

const (
	SLEEP_INTERVAL = 3600 * time.Second
)

var (
	CONSUMERS_PER_TOPIC int
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
	_, exists := os.LookupEnv("CONSUMERS_PER_TOPIC")
	CONSUMERS_PER_TOPIC = 10
	if exists == true {
		var err error
		CONSUMERS_PER_TOPIC, err = strconv.Atoi(os.Getenv("CONSUMERS_PER_TOPIC"))
		if err != nil {
			log.Printf("Failed to read CONSUMERS_PER_TOPIC env variable. Defaulting to 10.")
		}
	}
	log.Printf("Spawning %d consumers per topic", CONSUMERS_PER_TOPIC)

	// Utilizes goroutines to create concurrent Kafka Consumers.
	go consumerFactory(kafkaURL, groupID)

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

// Utilizes goroutines to create concurrent Kafka Consumers.
func consumerFactory(kafkaURL string, groupID string) {
	addChannel := make(chan int, CONSUMERS_PER_TOPIC)
	deleteChannel := make(chan int, CONSUMERS_PER_TOPIC)
	scrapeChannel := make(chan int, CONSUMERS_PER_TOPIC)
	go consumerManager(addChannel, kafkaURL, kafka.ADD_TOPIC, groupID)
	go consumerManager(deleteChannel, kafkaURL, kafka.DELETE_TOPIC, groupID)
	go consumerManager(scrapeChannel, kafkaURL, kafka.SCRAPE_TOPIC, groupID)
}

// Utilizes channels to maintain a set number of consumers per topic.
// Will wait 5 minutes prior to respawning a consumer.
func consumerManager(ch chan int, kafkaURL string, topic string, groupID string) {
	for i := 0; i < CONSUMERS_PER_TOPIC; i++ {
		go kafka.SpawnConsumer(ch, kafkaURL, topic, groupID)
	}
	for {
		<-ch
		log.Printf("Spawning new consumer in 5 minutes...")
		go func() {
			time.Sleep(time.Second * time.Duration(300))
			go kafka.SpawnConsumer(ch, kafkaURL, topic, groupID)
		}()
	}
}
