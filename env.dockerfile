FROM ubuntu:latest

ARG GolangciVersion=v1.50.1
ARG RedisVersion=7.0.6

ENV GOLANG_VERSION 1.19.4
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

RUN set -eux; \
    # install golang \
    wget -qO- https://dl.google.com/go/go1.19.4.linux-amd64.tar.gz | tar xvz --strip-components=1 -C /usr/local; \
    mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"; \
    \
    # install golangci-lint
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b /bin $GolangciVersion; \
    \
    # install redis
    mkdir -p /redis; \
    wget -qO- https://download.redis.io/releases/redis-$RedisVersion.tar.gz | tar xvz --strip-components=1 -C /redis; \
    cd /redis && make all; \
    rm -rf \
      /redis/deps \
      /redis/src/redis-benchmark \
      /redis/src/redis-check-aof \
      /redis/src/redis-check-rdb \
      /redis/src/redis-cli \
      /redis/src/redis-sentinel \
    ; \
    \
    mkdir -p /opt/6379/data; \
    cp /redis/redis.conf /opt/6379/; \
    \
    useradd grte -U -m

CMD ["/redis/src/redis-server", "/opt/6379/redis.conf", "--port", "6379", "--dir", "/opt/6379/data"]
