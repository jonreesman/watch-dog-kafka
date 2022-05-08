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

	"github.com/gin-gonic/gin"
	"github.com/jonreesman/wdk/db"
	"github.com/jonreesman/wdk/kafka"
	"github.com/jonreesman/wdk/pb"
)

type Server struct {
	d        db.DBManager
	router   *gin.Engine
	kafkaURL string
}

func NewServer(kafkaURL string) (*Server, error) {
	var s Server
	var err error
	s.d, err = db.NewManager(db.SLAVE)
	if err != nil {
		log.Printf("Error Opening DB connection in NewServer(): %v", err)
		return nil, err
	}
	s.router = gin.Default()
	s.kafkaURL = kafkaURL

	api := s.router.Group("/api")
	{
		api.GET("/", func(c *gin.Context) {
			c.HTML(
				http.StatusOK,
				"web/web.html",
				gin.H{"title": "Web"},
			)
			c.JSON(http.StatusOK, gin.H{
				"message": "pong",
			})
		})
		api.GET("/tickers", s.returnTickersHandler)
		api.POST("/tickers/", s.newTickerHandler)
		api.GET("/tickers/:id/time/:interval", s.returnTickerHandler)
		api.DELETE("/tickers/:id", s.deactivateTickerHandler)
	}
	return &s, nil
}

func (s *Server) startServer() error {
	err := s.router.Run(":3100")
	return err
}

func errorResponse(err error) gin.H {
	return gin.H{"error": err.Error()}
}

func (s Server) newTickerHandler(c *gin.Context) {
	var input db.Ticker
	fmt.Println(c.Request.Body)
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err))
		return
	}
	fmt.Println(input.Name)

	//CHECK IF REAL THEN MAKE PRODUCER

	if !CheckTickerExists(input.Name) {
		c.JSON(http.StatusNotFound, gin.H{"Id:": 0, "Name": "None"})
	}

	kafka.ProducerHandler(c, s.kafkaURL, kafka.ADD_TOPIC, input.Name)
}

func (s Server) returnTickersHandler(c *gin.Context) {
	tickers := s.d.ReturnActiveTickers()
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
	case "week":
		period = "7d"
	case "month":
		period = "30d"
	case "2month":
		period = "60d"
	}

	if tick, err = s.d.RetrieveTickerById(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to retrieve ticker"})
		return
	}

	sentimentHistory := s.d.ReturnSentimentHistory(id, fromTime)
	addr := "localhost:9999"
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock())
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
