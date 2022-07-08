# syntax=docker/dockerfile:1

FROM --platform=linux/amd64 golang:1.18-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . ./


RUN go build -o /main

CMD [ "/main" ]