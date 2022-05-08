#!/usr/bin/env bash

# kafka-demo topics
docker-compose exec kafka kafka-topics --create --bootstrap-server localhost:9093 --replication-factor 1 --partitions 10 --topic add
docker-compose exec kafka kafka-topics --create --bootstrap-server localhost:9093 --replication-factor 1 --partitions 10 --topic delete
docker-compose exec kafka kafka-topics --create --bootstrap-server localhost:9093 --replication-factor 1 --partitions 10 --topic scrape

