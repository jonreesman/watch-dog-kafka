package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/jonreesman/watch-dog-kafka/by"
	"github.com/jonreesman/watch-dog-kafka/cleaner"
	"github.com/jonreesman/watch-dog-kafka/consumer"
	"github.com/jonreesman/watch-dog-kafka/db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	SLEEP_INTERVAL = 3600 * time.Second
)

var (
	CONSUMERS = 5
)

// Run is our central loop that signals hourly to scrape for
// our active stock tickers and cryptocurrencies. Every hour,
// it creates a message on our Kafka `scrape` topic, which
// signals to our consumers to scrape for that stock/crypto.
func run(db db.DBManager, consumerChannel chan []string) error {
	// Grabs all active stock tickers every hour, and generates
	// a scrape message on the `scrape` Kafka topic.

	for {
		tickers, _ := db.ReturnActiveTickers(nil)
		go func() {
			for _, ticker := range tickers {
				log.Printf("Ticker %s", ticker.Name)
				go consumer.ProducerHandler(nil, consumerChannel, consumer.SCRAPE_TOPIC, ticker.Name)
			}
		}()
		time.Sleep(SLEEP_INTERVAL)
	}
}

func main() {
	// Grab all our environment variables.
	// Database environment variables.
	log.Printf("Starting")
	dbUser := os.Getenv("DB_USER")
	dbPwd := os.Getenv("DB_PWD")
	dbName := os.Getenv("DB_NAME")
	dbMasterURL := os.Getenv("DB_MASTER")
	dbSlaveURL := os.Getenv("DB_SLAVE")

	// GRPC environment variable
	grpcHost := os.Getenv("GRPC_HOST")

	if _, exists := os.LookupEnv("CONSUMERS_PER_TOPIC"); exists {
		var err error
		CONSUMERS, err = strconv.Atoi(os.Getenv("CONSUMERS_PER_TOPIC"))
		if err != nil {
			log.Printf("Failed to read CONSUMERS_PER_TOPIC env variable. Defaulting to 10.")
		}
	}

	// Set up our pprof server
	/*go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()*/
	log.Printf("Connecting to database...\n")
	main, err := db.NewManager(dbUser, dbPwd, dbName, dbMasterURL)
	if err != nil {
		log.Fatalf("Error Opening DB connection in NewServer(): %v", err)
	}
	log.Printf("Connecting to replica database...\n")
	replica, err := db.NewManager(dbUser, dbPwd, dbName, dbSlaveURL)
	if err != nil {
		log.Fatalf("Error Opening DB connection in NewServer(): %v", err)
	}
	log.Printf("Connected.\nConnecting to gRPC server...")

	grpcServerConn, err := grpc.Dial(grpcHost, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		log.Fatalf("main(): Failed to dial GRPC.")
		return
	}
	log.Printf("Connected.\n Loading spam detection model...")
	spamDetector, err := by.LoadModelFromFile("model.by")
	if err != nil {
		log.Fatalf("main(): Failed to load spam detection model %v", err)
	}

	cleaner := cleaner.NewCleaner()

	consumerConfig := consumer.ConsumerConfig{
		DbManager:      main,
		GrpcServerConn: grpcServerConn,
		SpamDetector:   &spamDetector,
		Cleaner:        cleaner,
	}

	log.Printf("Spawning consumers...\n")
	consumerChannel := make(chan []string, CONSUMERS)
	for i := 0; i < CONSUMERS; i++ {
		go consumer.SpawnConsumer(consumerChannel, consumerConfig)
	}

	// Grabs an instance of our Gin server, passing the kafkaURL.
	// Gin server requires the KafkaURL so that it can create
	// its own Kafka producers.
	s, err := NewServer(replica, grpcServerConn, consumerChannel)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting webserver...\n")
	// Fails and aborts if the Gin server fails to launch,
	// since there is no reason to run without the API.
	go s.startServer()

	// Launches the hourly loop that results in a regular
	// scraping for each stock ticker/crypto. If this fails,
	// we will also abort.
	go run(replica, consumerChannel)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}
