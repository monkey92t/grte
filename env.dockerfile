FROM ubuntu:20.04

ARG GolangciVersion=v1.41.1
ARG RedisVersion=6.2.5

ENV GOLANG_VERSION 1.16.7
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
    wget -qO- https://dl.google.com/go/go1.16.7.linux-amd64.tar.gz | tar xvz --strip-components=1 -C /usr/local; \
    mkdir -p "$GOPATH/src" "$GOPATH/bin" && chmod -R 777 "$GOPATH"; \
    \
    # install golangci-lint
    curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b /bin $GolangciVersion; \
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
