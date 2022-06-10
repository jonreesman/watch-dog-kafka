package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"google.golang.org/grpc"

	_ "net/http/pprof"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jonreesman/watch-dog-kafka/db"
	"github.com/jonreesman/watch-dog-kafka/kafka"
	"github.com/jonreesman/watch-dog-kafka/pb"
)

type Server struct {
	d        db.DBManager
	router   *gin.Engine
	grpcHost string
	kafkaURL string
}

// Creates and returns a server instance to main.
// This server struct contains an instance of our
// database manager, the Gin router, and the kafkaURL
// so that it can produce messages in our Kafk topics.
func NewServer(kafkaURL, grpcHost string) (*Server, error) {
	var (
		s   Server
		err error
	)

	// Generates a database manager on our slave/replica database
	// since we will only need read access.
	s.d, err = db.NewManager(db.SLAVE)
	if err != nil {
		log.Printf("Error Opening DB connection in NewServer(): %v", err)
		return nil, err
	}
	s.router = gin.Default()
	s.router.Use(cors.Default())
	s.kafkaURL = kafkaURL
	s.grpcHost = grpcHost

	// Basic routing to generate our REST API handlers.
	api := s.router.Group("/api")
	{
		api.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "pong",
			})
		})
		api.GET("/tickers", s.returnTickersHandler)
		api.GET("/tickers/:id/time/:interval", s.returnTickerHandler)
	}
	auth := s.router.Group("/auth")
	{
		auth.POST("/tickers/", s.newTickerHandler)
		auth.DELETE("/tickers/:id", s.deactivateTickerHandler)
	}
	return &s, nil
}

func (s *Server) startServer() error {
	err := s.router.Run(":3100")
	return err
}

// This errorResponse handler is deprecated and will be removed.
func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}

// Recieves a stock ticker name as a string via a POST request
// then validates whether it is a valid ticker prior to publishing
// it to be added and scraped on the `add` Kafka topic.
/*
	POST Request Form: http://[ip]:[port]/auth/tickers/
	Request Body (JSON): "name": "[ticker name]"
	Response Form:
		"success": true
*/
func (s Server) newTickerHandler(c *gin.Context) {
	var input db.Ticker
	fmt.Println(c.Request.Body)
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	sanitizedTicker := SanitizeTicker(input.Name)

	if !CheckTickerExists(sanitizedTicker) {
		c.JSON(http.StatusNotFound, gin.H{"Id:": 0, "Name": "None"})
	}

	kafka.ProducerHandler(c, s.kafkaURL, kafka.ADD_TOPIC, sanitizedTicker)
}

// Returns only active tickers when called with a GET request.
/*
	GET Request Form: http://[ip]:[port]/api/tickers/
	Response Form:
		ticker: {
			Name: [name],
			LastScrapeTime: [lastScrapeTime (UNIX)],
			HourlySentiment: [current hourly senitment],
			Id: [ticker ID from database],
			Quote: [current stock price]
		}
*/
func (s Server) returnTickersHandler(c *gin.Context) {
	tickers, err := s.d.ReturnActiveTickers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, nil)
	}
	//Add current prices to tickers
	type payloadItem struct {
		Name            string
		LastScrapeTime  time.Time
		HourlySentiment float64
		Id              int
		Quote           float64
	}
	payload := make([]payloadItem, 0)

	for _, tick := range tickers {
		it := payloadItem{
			Name:            tick.Name,
			LastScrapeTime:  tick.LastScrapeTime,
			HourlySentiment: tick.HourlySentiment,
			Id:              tick.Id,
			Quote:           priceCheck(tick.Name),
		}
		payload = append(payload, it)
	}

	c.JSON(http.StatusOK, payload)
}

// Returns all the data contained for a stock ticker specified by ID
// as a param via GET request. It will gather all tweets, hourly sentiment
// averages, and quotes for a given timespan and return it.
/*
	Request Form: http://[ip]:[port]/api/tickers/{id}/time/{timespan}
	Valid `timespans`: [`day`, `week`, `month`, `2month`]
	Response Form:
		TO DO
*/
func (s Server) returnTickerHandler(c *gin.Context) {
	var (
		id       int
		interval string
		fromTime int64
		t        db.Ticker
		name     string
		period   string
		tick     db.Ticker
		err      error
	)
	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id."})
		return
	}
	t, err = s.d.RetrieveTickerById(id)
	if err != nil {
		log.Print("Unable to retieve ticker")
	}
	name = t.Name

	interval = c.Param("interval")
	switch interval {
	case "day":
		period = "1d"
		fromTime = time.Now().Unix() - 86400
	case "week":
		period = "7d"
		fromTime = time.Now().Unix() - 86400*7
	case "month":
		period = "30d"
		fromTime = time.Now().Unix() - 86400*30
	case "2month":
		period = "60d"
		fromTime = time.Now().Unix() - 86400*60

	}

	if tick, err = s.d.RetrieveTickerById(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to retrieve ticker"})
		return
	}

	sentimentHistory := s.d.ReturnSentimentHistory(id, fromTime)
	conn, err := grpc.Dial(s.grpcHost, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Printf("returnTickerHandler(): GRPC Dial Error %v", err)
		errorResponse(err)
		return
	}
	defer conn.Close()
	client := pb.NewQuotesClient(conn)
	request := pb.QuoteRequest{
		Name:   name,
		Period: period,
	}
	response, err := client.Detect(context.Background(), &request)
	if err != nil {
		log.Printf("returnTickerHandler(): GRPC Detect Error: %v", err)
		c.JSON(http.StatusBadRequest, errorResponse(errors.New("Ticker symbol could not be found. If crypto, please try with the relative currency (eg. BTC-USD).")))
		return
	}
	quoteHistory := make([]db.IntervalQuote, 0)
	for _, quote := range response.Quotes {
		quoteHistory = append(quoteHistory, db.IntervalQuote{TimeStamp: quote.Time.Seconds, CurrentPrice: float64(quote.Price)})
	}

	statementHistory := s.d.ReturnAllStatements(id, fromTime)

	c.JSON(http.StatusOK, gin.H{
		"ticker":            tick,
		"quote_history":     quoteHistory,
		"sentiment_history": sentimentHistory,
		"statement_history": statementHistory,
	})
}

// Deactivates the ticker for hourly scraping and display
// by generating a `DELETE` message on the `delete` Kafka topic.
/*
	DELETE Request Form: http://[ip]:[port]/auth/tickers/{id}
	Response Form:
		"success": true
		"error": "Invalid id."
*/
func (s Server) deactivateTickerHandler(c *gin.Context) {
	var (
		id  int
		err error
	)
	if id, err = strconv.Atoi(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id."})
		return
	}

	kafka.ProducerHandler(c, s.kafkaURL, kafka.DELETE_TOPIC, strconv.Itoa(id))
}
