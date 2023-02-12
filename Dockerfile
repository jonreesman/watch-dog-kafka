# syntax=docker/dockerfile:1

FROM --platform=linux/amd64 golang:1.19-buster
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o main


FROM --platform=amd64 python:3.8-slim
WORKDIR /app
COPY --from=0 /app /app
RUN chmod +x /app/startup.sh
RUN pip install -r /app/py/requirements.txt
RUN ln -s /lib/ld-musl-x86_64.so.1 /lib/ld-linux-x86-64.so.2

CMD [ "/app/startup.sh" ]