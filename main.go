package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/jonreesman/watch-dog-kafka/by"
	"github.com/jonreesman/watch-dog-kafka/cleaner"
	"github.com/jonreesman/watch-dog-kafka/db"
	"github.com/jonreesman/watch-dog-kafka/kafka"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	SLEEP_INTERVAL = 3600 * time.Second
)

var (
	CONSUMERS_PER_TOPIC = 5
)

// Run is our central loop that signals hourly to scrape for
// our active stock tickers and cryptocurrencies. Every hour,
// it creates a message on our Kafka `scrape` topic, which
// signals to our consumers to scrape for that stock/crypto.
func run(db db.DBManager, kafkaURL string) error {
	// Grabs all active stock tickers every hour, and generates
	// a scrape message on the `scrape` Kafka topic.

	//Give Kafka time to start up.
	log.Printf("Waiting 5 minutes to start initial scrape...")
	time.Sleep(5 * time.Minute)
	log.Printf("5 minutes elapsed... starting scrape.")
	for {
		tickers, _ := db.ReturnActiveTickers(nil)
		go func() {
			for _, ticker := range tickers {
				log.Printf("Ticker %s", ticker.Name)
				go kafka.ProducerHandler(nil, kafkaURL, kafka.SCRAPE_TOPIC, ticker.Name)
			}
		}()
		time.Sleep(SLEEP_INTERVAL)
	}
}

func main() {
	// Grab all our environment variables.
	// Database environment variables.
	dbUser := os.Getenv("DB_USER")
	dbPwd := os.Getenv("DB_PWD")
	dbName := os.Getenv("DB_NAME")
	dbMasterURL := os.Getenv("DB_MASTER")
	dbSlaveURL := os.Getenv("DB_SLAVE")

	// GRPC environment variable
	grpcHost := os.Getenv("GRPC_HOST")

	// Kafka environment variables.
	kafkaURL := os.Getenv("kafkaURL")
	groupID := os.Getenv("groupID")
	if _, exists := os.LookupEnv("CONSUMERS_PER_TOPIC"); exists {
		var err error
		CONSUMERS_PER_TOPIC, err = strconv.Atoi(os.Getenv("CONSUMERS_PER_TOPIC"))
		if err != nil {
			log.Printf("Failed to read CONSUMERS_PER_TOPIC env variable. Defaulting to 10.")
		}
	}

	// Set up our pprof server
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	main, err := db.NewManager(dbUser, dbPwd, dbName, dbMasterURL)
	if err != nil {
		log.Fatalf("Error Opening DB connection in NewServer(): %v", err)
	}
	replica, err := db.NewManager(dbUser, dbPwd, dbName, dbSlaveURL)
	if err != nil {
		log.Fatalf("Error Opening DB connection in NewServer(): %v", err)
	}
	grpcServerConn, err := grpc.Dial(grpcHost, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		log.Fatalf("main(): Failed to dial GRPC.")
		return
	}

	spamDetector, err := by.LoadModelFromFile("model.by")
	if err != nil {
		log.Fatalf("main(): Failed to load spam detection model %v", err)
	}

	cleaner := cleaner.NewCleaner()

	consumerConfig := kafka.ConsumerConfig{
		DbManager:      main,
		GrpcServerConn: grpcServerConn,
		SpamDetector:   &spamDetector,
		Cleaner: 		cleaner,
	}

	// Utilizes goroutines to create concurrent Kafka Consumers.
	go consumerFactory(consumerConfig, kafkaURL, groupID)

	// Grabs an instance of our Gin server, passing the kafkaURL.
	// Gin server requires the KafkaURL so that it can create
	// its own Kafka producers.
	s, err := NewServer(replica, grpcServerConn, kafkaURL)
	if err != nil {
		log.Fatal(err)
	}

	// Fails and aborts if the Gin server fails to launch,
	// since there is no reason to run without the API.
	go s.startServer()

	// Launches the hourly loop that results in a regular
	// scraping for each stock ticker/crypto. If this fails,
	// we will also abort.
	go run(replica, kafkaURL)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}

// Utilizes goroutines to create concurrent Kafka Consumers.
func consumerFactory(config kafka.ConsumerConfig, kafkaURL string, groupID string) {
	addChannel := make(chan int, CONSUMERS_PER_TOPIC)
	deleteChannel := make(chan int, CONSUMERS_PER_TOPIC)
	scrapeChannel := make(chan int, CONSUMERS_PER_TOPIC)
	go consumerManager(addChannel, config, kafkaURL, kafka.ADD_TOPIC, groupID)
	go consumerManager(deleteChannel, config, kafkaURL, kafka.DELETE_TOPIC, groupID)
	go consumerManager(scrapeChannel, config, kafkaURL, kafka.SCRAPE_TOPIC, groupID)
}

// Utilizes channels to maintain a set number of consumers per topic.
// Will wait 5 minutes prior to respawning a consumer.
func consumerManager(ch chan int, config kafka.ConsumerConfig, kafkaURL string, topic string, groupID string) {
	for i := 0; i < CONSUMERS_PER_TOPIC; i++ {
		go kafka.SpawnConsumer(ch, config, kafkaURL, topic, groupID)
	}
	for {
		<-ch
		log.Printf("Spawning new consumer in 5 minutes...")
		go func() {
			time.Sleep(time.Second * time.Duration(300))
			go kafka.SpawnConsumer(ch, config, kafkaURL, topic, groupID)
		}()
	}
}
