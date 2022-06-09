# watch-dog - Kafka Edition
The purpose of this project is to scrape twitter, reddit, and various news sources on an hourly basis in order to provide the user with a regular sentiment analysis for their chosen stock/crypto tickers. It has an accompanying Next.js based frontend located [here.](https://github.com/jonreesman/watch-dog-next) You can find a live demo of my own instance [here.](https://watch-dog.jonreesman.dev).

There is a deprecated React Native version listed [here](https://github.com/jonreesman/watch-dog-react). The original, non event-driven approach, is located [here.](https://github.com/jonreesman/watch-dog)

## Set-Up
1. Use `docker-compose.yml` to "compose up" the Kafka, Zookeeper, MySQL main and replica databases, backend, python GRPC server, proxy server, and proxy server mongodb database.
    - ENV variables: This program operates off env variables that you must set. They are as follows...
        - "kafkaURL": "localhost:9093",
        - "groupID": "demo",
        - "GRPC_HOST": "localhost:9999",
        - "DB_MASTER": "3306",
        - "DB_SLAVE": "3307",
        - "DB_USER": "root",
        - "DB_PWD": "password",
        - "DB_NAME": "app"
        - "CONSUMERS_PER_TOPIC": 10 {default}
    - Often times, the MySQL configuration fails, resulting in a replica database that is out of sync with the main database. To remove the replication, simply change the parameter in NewServer() from `db.SLAVE` to `db.MASTER`.
2. [OPTIONAL] From the commandline, use `sh createTopics.sh` to set up the Kafka Topics. This step is optional, as the consumers will make the topics for you.


## Kafka
This version of watch-dog leverages Kafka to make it horizonally scalable. This is intended to be a microservice version. It is still very elementary in application and is actually slower when used by a small number of users. To make it truly applicable to a wider crowd, I will need to implement a user system, to allow custom stock/crypto ticker lists. Presently, its one monolithic selection for all users and has no authentication.

## Front-end
The frontend currently serves as a display for the stocks the program is already tracking. I am in the process of adding authentication, so the frontend only accesses GET requests from the API via the jwt-auth-proxy. I am working on implementing an authentication system that will allow users to log on and add stocks through the website.

## Backend
At the beginning of this project, Go had full responsibility for the backend. As time goes on, I've slowly fractured some of the functionalities and implemented them in Python. Go presently is responsible for networking using [Gin](https://github.com/gin-gonic/gin) as well as scraping Twitter using a [Frontend scraper written in Go](https://github.com/n0madic/twitter-scraper) (shoutout to n0madic). The actual Sentiment Analysis and pulling of stock and crypto quotes (the Yahoo Finance API is slowly falling apart and Go packages are casualties) is handled with Python, which communicates with Go via GRPC.

## Database
I explored NoSQL implementations like DynamoDB and MongoDB, but ultimately settled for MySQL. It's tried and true, and I presently don't require the flexibility of NoSQL. As I learn more about Software Engineering however, I find that NoSQL may be a necessity for properly scaling this project should it shift to a centrally run service.

## The Way Forward
- [ ] Reddit Scraping
- [ ] News Scraping

