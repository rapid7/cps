FROM golang:1.10.3-alpine3.7

WORKDIR /go/src/github.com/rapid7/cps

COPY . /go/src/github.com/rapid7/cps

RUN apk add --update-cache git && \
  mkdir -p /etc/cps && \
  go get -u github.com/golang/dep/cmd/dep && \
  export GOPATH=/go && \
  dep ensure -v && make build && mv cps /
  
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
