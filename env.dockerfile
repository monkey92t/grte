FROM ubuntu:latest

ARG GolangciVersion=v1.50.1
ARG RedisVersion=7.0.9

ENV GOLANG_VERSION 1.20.2
ENV GOPATH /go
ENV PATH /usr/local/go/bin:$GOPATH/bin:$PATH

RUN apt-get update && apt-get install -y --no-install-recommends \
	g++ \
	gcc \
	libc6-dev \
	make \
        curl \
        wget \
        openssl \
        ca-certificates \
	pkg-config \
    && rm -rf /var/lib/apt/lists/*

# install golang
RUN set -eux; \
    wget -qO- https://dl.google.com/go/go1.20.2.linux-amd64.tar.gz | tar xvz --strip-components=1 -C /usr/local; \
    mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"

# install golangci-lint
RUN curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /bin $GolangciVersion

# install redis
RUN set -eux; \
    mkdir -p /redis; \
    wget -qO- https://download.redis.io/releases/redis-$RedisVersion.tar.gz | tar xvz --strip-components=1 -C /redis; \
    cd /redis && make all; \
    \
    mkdir -p /opt/6379/data; \
    cp /redis/redis.conf /opt/6379/

RUN useradd grte -U -m

CMD ["/redis/src/redis-server", "/opt/6379/redis.conf", "--port", "6379", "--dir", "/opt/6379/data"]
