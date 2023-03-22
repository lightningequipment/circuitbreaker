# Install dependencies only when needed
FROM node:lts-alpine AS build_frontend
# Check https://github.com/nodejs/docker-node/tree/b4117f9333da4138b03a546ec926ef50a31506c3#nodealpine to understand why libc6-compat might be needed.
RUN apk add --no-cache libc6-compat
WORKDIR /app
COPY web .
RUN yarn install --frozen-lockfile && yarn build-export

### Build backend
FROM golang:1.19-alpine AS build_backend

ARG BUILD_VERSION

WORKDIR /src

COPY *.go go.mod go.sum ./
COPY circuitbreakerrpc circuitbreakerrpc/
COPY --from=build_frontend /webui-build/ webui-build/

RUN go install -ldflags "-X main.BuildVersion=$BUILD_VERSION"

### Build an Alpine image
FROM alpine:3.16 as alpine

# Update CA certs
RUN apk add --no-cache ca-certificates && rm -rf /var/cache/apk/*

# Copy over app binary
COPY --from=build_backend /go/bin/circuitbreaker /usr/bin/circuitbreaker

ENTRYPOINT [ "circuitbreaker" ]
