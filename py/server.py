import logging
import math
from concurrent.futures import ThreadPoolExecutor

import grpc
from pydantic import BaseModel
from textblob import TextBlob 

from google.protobuf.timestamp_pb2 import Timestamp
import yfinance as yf

from watchdog_pb2 import SentimentResponse
from watchdog_pb2 import QuoteResponse
from watchdog_pb2_grpc import SentimentServicer, add_SentimentServicer_to_server
from watchdog_pb2_grpc import QuotesServicer, add_QuotesServicer_to_server


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
            posixTime = tuple[0].timestamp()
            seconds = math.floor(posixTime)
            protoTime = Timestamp(seconds=seconds)
            resp.quotes.add(time=protoTime, price=tuple[1])
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
    server.add_insecure_port(f'[::]:{port}')
    server.start()
    logging.info('server reads on port %r', port)
    server.wait_for_termination()