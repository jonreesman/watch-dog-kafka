package kafka

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

func ProducerHandler(c *gin.Context, kafkaURL, topic, ticker string) {
	kafkaWriter := getKafkaWriter(kafkaURL, topic)
	defer kafkaWriter.Close()
	msg := kafka.Message{
		Key:   []byte(uuid.New().String()),
		Value: []byte(ticker),
	}

	if c == nil {
		if err := kafkaWriter.WriteMessages(context.Background(), msg); err != nil {
			log.Printf("ProducerHandler failed to write message for ticker %s: %v\n",ticker, err)
		}
		return
	}

	err := kafkaWriter.WriteMessages(c.Request.Context(), msg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func getKafkaWriter(kafkaURL, topic string) *kafka.Writer {
	return &kafka.Writer{
		Addr:     kafka.TCP(kafkaURL),
		Topic:    topic,
		Balancer: &kafka.LeastBytes{},
	}
}
