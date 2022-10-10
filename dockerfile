# syntax=docker/dockerfile:1

FROM --platform=linux/amd64 golang:1.18-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

RUN apk update
RUN apk add python3
RUN apk add py3-pip

RUN pip3 install  --upgrade setuptools
RUN pip3 install --upgrade pip
RUN pip3 install -r ./py/requirements.txt

RUN go build -o /main
RUN python3 ./py/server.py

CMD [ "/main" ]