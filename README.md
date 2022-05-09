# watch-dog - Kafka Edition
The purpose of this project is to scrape twitter, reddit, and various news sources on an hourly basis in order to provide the user with a regular sentiment analysis for their chosen stock/crypto tickers. It's React Native frontend component can currently be found [here](https://github.com/jonreesman/watch-dog-react) which is where the lionshare of my attention presently is focused.

## Set-Up
1. Use `docker-compose.yml` to "compose up" the Kafka, Zookeeper, and MySQL main and replica databases.
    - ENV variables: This program operates off a set of env variables that you must set. They are as follows...
        - "kafkaURL": "localhost:9093",
        - "groupID": "demo",
        - "DB_MASTER_PORT": "3307",
        - "DB_SLAVE_PORT": "3308",
        - "DB_USER": "root",
        - "DB_PWD": "password",
        - "DB_NAME": "app"
    - Often times, the MySQL configuration fails, resulting in a replica database that is out of sync with the main database. To remove the replication, simply change the parameter in NewServer() from `db.SLAVE` to `db.MASTER`.
2. From the commandline, use `sh createTopics.sh` to set up the Kafka Topics.
3. From the commandline, use `python3 py/server.py` to set up the Python microservice responsible for sentiment analysis and pulling stock quotes.
4. Finally, build and run the main binary with `go build`.


## Kafka
This version of watch-dog leverages Kafka to make it horizonally scalable. This is intended to be a microservice version. It is still very elementary in application and is actually slower when used by a small number of users. To make it truly applicable to a wider crowd, I will need to implement a user system, to allow custom stock/vrypto ticker lists. Presently, its one monolithic selection for all users and has no authentication.

## Front-end
The entire app is capable of adding and removing stocks/crypto on the frontend, displaying the tweets it is currently using as datapoints, and a dual axes graph of the average hourly sentiment analysis overlayed with the price for the same time range. Presently, I have utilized React Native, but will likely shift in the future to try out different frameworks for my own edification.

## Backend
At the beginning of this project, Go had full responsibility for the backend. As time goes on, I've slowly fractured some of the functionalities and implemented them in Python. Go presently is responsible for networking using [Gin](https://github.com/gin-gonic/gin) as well as scraping Twitter using a [Frontend scraper written in Go](https://github.com/n0madic/twitter-scraper) (shoutout to n0madic). The actual Sentiment Analysis and pulling of stock and crypto quotes (the Yahoo Finance API is slowly falling apart and Go packages are casualties) is handled with Python, which communicates with Go via GRPC.

## Database
I explored NoSQL implementations like DynamoDB and MongoDB, but ultimately settled for MySQL. It's tried and true, and I presently don't require the flexibility of NoSQL. As I learn more about Software Engineering however, I find that NoSQL may be a necessity for properly scaling this project should it shift to a centrally run service.

## The Way Forward
- [ ] Reddit Scraping
- [ ] News Scraping

The watch-dog project is my first for Go, so it is a learning curve for me.

