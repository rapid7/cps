FROM golang:1.16.3-alpine3.13

WORKDIR /src
COPY . /src
RUN apk add --update-cache git make && \
    mkdir /etc/cps && \
    make build && \
    mv cps /cps

FROM alpine:latest

WORKDIR /

COPY --from=0 /cps .
ADD dockerfiles/cps.json /
ADD dockerfiles/services/ /services
RUN apk add --update-cache ca-certificates && \
  touch /usr/bin/ec2metadata && mkdir -p /go/src/cps
COPY . /go/src/cps

EXPOSE 9100/tcp

CMD ["/cps"]
