FROM alpine:latest

WORKDIR /src
COPY . /src
RUN apk add --update-cache git make && \
    apk add go=1.16.7-r0 && \
    wget https://releases.hashicorp.com/consul/1.2.2/consul_1.2.2_linux_amd64.zip && \
    unzip consul_1.2.2_linux_amd64.zip && \
    mv consul /usr/bin/

CMD ["make", "test"]
