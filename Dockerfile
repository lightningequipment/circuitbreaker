FROM golang:alpine3.17 AS build-env

RUN apk add --update --no-cache git

COPY .  /circuitbreaker 

RUN cd /circuitbreaker \
    && rm -rf .git \
    && go install
RUN chmod a+x $GOPATH/bin/circuitbreaker

FROM alpine:3.17

LABEL org.opencontainers.image.source https://github.com/lightningequipment/circuitbreaker

COPY --from=build-env /go/bin/circuitbreaker /circuitbreaker

VOLUME [ "/root/.circuitbreaker" ]
WORKDIR /

ENTRYPOINT ["/circuitbreaker"]

