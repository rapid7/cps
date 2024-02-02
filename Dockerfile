ARG JOB_NAME
ARG GIT_REPO
ARG MAINTAINER

FROM golang:1.21.3-alpine

LABEL MAINTAINER=$MAINTAINER
LABEL JOB_NAME=$JOB_NAME
LABEL GIT_REPO=$GIT_REPO

WORKDIR /src
COPY . /src
RUN apk add --update-cache git make && \
    mkdir /etc/cps && \
    make build && \
    mv cps /cps

FROM alpine:latest

ENV AWS_ACCESS_KEY_ID=
ENV AWS_SECRET_ACCESS_KEY=
ENV AWS_SESSION_TOKEN=

WORKDIR /

COPY --from=0 /cps .
# Local testing
ADD dockerfiles/cps.json /
# ADD dockerfiles/services/ /services
RUN apk add --update-cache ca-certificates && \
  touch /usr/bin/ec2metadata && mkdir -p /go/src/cps
COPY . /go/src/cps

EXPOSE 9100/tcp

CMD ["/cps"]
