FROM golang:1.16

RUN apt-get update && \
  apt-get install -y wget make unzip && \
  ls

COPY Makefile .

RUN  make protoc-buf protoc-tools protoc-go-tools