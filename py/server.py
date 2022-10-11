import logging
import math
from concurrent.futures import ThreadPoolExecutor

import grpc
from pydantic import BaseModel
from textblob import TextBlob 

from google.protobuf.timestamp_pb2 import Timestamp
import yfinance as yf

from google import auth as google_auth
from google.auth.transport import grpc as google_auth_transport_grpc
from google.auth.transport import requests as google_auth_transport_requests

from watchdog_pb2 import SentimentResponse
from watchdog_pb2 import QuoteResponse
from watchdog_pb2_grpc import SentimentServicer, add_SentimentServicer_to_server
from watchdog_pb2_grpc import QuotesServicer, add_QuotesServicer_to_server

# Given a string, `tweet`, returns the sentiment analysis.
def find_sentiment(tweet):
    blob = TextBlob(tweet)
    return blob.sentiment.polarity

class SentimentServer(SentimentServicer):
    def Detect(self, request, context):
        logging.info('detect request size: %d', len(request.tweet))
        resp = SentimentResponse(polarity=find_sentiment(request.tweet))
        return resp

class QuotesServer(QuotesServicer):
    def Detect(self, request, context):
        logging.info('detect request size: %d', len(request.name))
        data = yf.download(tickers=request.name, period=request.period, interval='1h')
        if (data.size == 0):
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, 'Ticker doesnt exist')
        print(data.size)
        data = data.drop(['High','Low','Close','Adj Close', 'Volume'], axis=1)
        resp = QuoteResponse()
        for tuple in data.itertuples():
            posix_time = tuple[0].timestamp()
            seconds = math.floor(posix_time)
            proto_time = Timestamp(seconds=seconds)
            resp.quotes.add(time=proto_time, price=tuple[1])
        return resp

if __name__ == "__main__":
    logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(levelname)s - %(message)s',
    )
    server = grpc.server(ThreadPoolExecutor())
    add_SentimentServicer_to_server(SentimentServer() ,server)
    add_QuotesServicer_to_server(QuotesServer() , server)
    port = 9999

    credentials, _ = google_auth.default()
    request = google_auth_transport_requests.Request()
    #channel = google_auth_transport_grpc.secure_authorized_channel(
    #credentials, request, 'greeter.googleapis.com:443')
    #server.add_insecure_port(f'[::]:{port}')
    #creds = grpc.ssl_server_credentials([(server_key, server_cert)])
    server.add_secure_port(f"[::]:{port}", credentials)
    server.start()
    logging.info('server reads on port %r', port)
    server.wait_for_termination()