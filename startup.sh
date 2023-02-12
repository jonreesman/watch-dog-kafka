#!/bin/sh

set -m
python /app/py/server.py &
sleep 20
/app/main 

fg %1
