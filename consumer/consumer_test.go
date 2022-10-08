package consumer

import (
	"log"
	"os"
	"testing"

	"github.com/jonreesman/watch-dog-kafka/by"
	"github.com/jonreesman/watch-dog-kafka/cleaner"
	"github.com/jonreesman/watch-dog-kafka/db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestSpawnConsumer(t *testing.T) {
	dbUser := os.Getenv("DB_USER")
	dbPwd := os.Getenv("DB_PWD")
	dbName := os.Getenv("DB_NAME")
	dbMasterURL := os.Getenv("DB_MASTER")

	// GRPC environment variable
	grpcHost := os.Getenv("GRPC_HOST")

	// Kafka environment variables.
	main, err := db.NewManager(dbUser, dbPwd, dbName, dbMasterURL)
	if err != nil {
		log.Fatalf("Error Opening DB connection in NewServer(): %v", err)
	}
	grpcServerConn, err := grpc.Dial(grpcHost, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		log.Fatalf("main(): Failed to dial GRPC.")
		return
	}

	spamDetector, err := by.LoadModelFromFile("../model.by")
	if err != nil {
		log.Fatalf("main(): Failed to load spam detection model %v", err)
	}

	cleaner := cleaner.NewCleaner()

	consumerConfig := ConsumerConfig{
		DbManager:      main,
		GrpcServerConn: grpcServerConn,
		SpamDetector:   &spamDetector,
		Cleaner:        cleaner,
	}

	addChannel := make(chan []string, 5)
	ProducerHandler(nil, addChannel, SCRAPE_TOPIC, "AMD")
	ProducerHandler(nil, addChannel, SCRAPE_TOPIC, "AAPL")
	ProducerHandler(nil, addChannel, SCRAPE_TOPIC, "AMC")

	SpawnConsumer(addChannel, consumerConfig)

}
