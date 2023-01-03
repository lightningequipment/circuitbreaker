### Build frontend
FROM node:15.4 as build_frontend

WORKDIR /react-app

COPY webui/package*.json .

RUN npm install

COPY webui .

RUN npm run build

### Build backend
FROM golang:1.18-alpine AS build_backend

WORKDIR /src

COPY *.go go.mod go.sum ./
COPY circuitbreakerrpc circuitbreakerrpc/
COPY --from=build_frontend /webui-build/ webui-build/

RUN go install .

### Build an Alpine image
FROM alpine:3.16 as alpine

# Update CA certs
RUN apk add --no-cache ca-certificates && rm -rf /var/cache/apk/*

# Copy over app binary
COPY --from=build_backend /go/bin/circuitbreaker /usr/bin/circuitbreaker

ENTRYPOINT [ "circuitbreaker" ]
