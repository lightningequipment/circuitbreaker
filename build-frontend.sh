#!/bin/bash

docker build --target build_frontend -t circuitbreaker-frontend-builder .
CID=$(docker create circuitbreaker-frontend-builder)
docker cp $CID:/webui-build .
docker rm -f $CID