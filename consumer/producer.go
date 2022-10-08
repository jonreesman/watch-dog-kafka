package consumer

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Middleware that creates a Kafka Writer and writes the given
// message (a stock/crypto ticker here) to the given topic.
// Has no return, but given the context, will return an HTTP response.
// This method also doubles as a non-middleware Kafka producer, so it will
// accept a `nil` context and write the given message to the topic anyways.
func ProducerHandler(c *gin.Context, consumerChannel chan []string, topic, ticker string) {
	if ticker == "" {
		return
	}

	if c == nil {
		go func() {
			consumerChannel <- []string{ticker, topic}
		}()
		return
	}

	consumerChannel <- []string{ticker, topic}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
