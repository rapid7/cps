FROM golang:1.10.3-alpine3.7

WORKDIR /go/src/cps

COPY . /go/src/cps

RUN apk add --update-cache git && \
  mkdir -p /etc/cps && \
  go get -u github.com/golang/dep/cmd/dep && \
  dep ensure -v && go build -o /cps -v && \
  rm -rf /go/src/cps/*

FROM alpine:latest

WORKDIR /

COPY --from=0 /cps .
ADD dockerfiles/cps.json /
ADD dockerfiles/services/ /services
RUN touch /usr/bin/ec2metadata

EXPOSE 9100/tcp

CMD ["/cps"]
