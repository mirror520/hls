FROM ubuntu:18.04

RUN apt update && apt upgrade -y \
 && apt install -y git wget

ENV GOLANG_VERSION 1.12
ENV goRelArch linux-amd64

RUN wget https://golang.org/dl/go${GOLANG_VERSION}.${goRelArch}.tar.gz \
 && tar -C /usr/local -xzf go${GOLANG_VERSION}.${goRelArch}.tar.gz \
 && rm go${GOLANG_VERSION}.${goRelArch}.tar.gz

ENV GOPATH /go
ENV PATH $GOPATH/bin:/usr/local/go/bin:$PATH

RUN go get github.com/gorilla/mux \
 && go get github.com/influxdata/influxdb1-client \
 && go get github.com/rs/cors \
 && go get github.com/mirror520/hls \
 && go install github.com/mirror520/hls

WORKDIR $GOPATH/src/github.com/mirror520/hls

EXPOSE 80

CMD hls