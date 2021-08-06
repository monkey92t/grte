FROM golang:1.16.6-alpine

# set golangci-lint version
ARG GolangciVersion=v1.41.1

# set redis-server version
ARG RedisVersion=6.2.5

RUN set -eux; \
    \
    apk add --no-cache curl make gcc g++; \
    \
    # install golangci-lint
    curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(go env GOPATH)/bin $GolangciVersion; \
    \
    # install redis-server
    mkdir -p /redis/src && mkdir -p /tmp/redis; \
    wget -qO- https://download.redis.io/releases/redis-$RedisVersion.tar.gz | tar xvz --strip-components=1 -C /tmp/redis; \
    cd /tmp/redis && \
        make all && \
        cp redis.conf /redis/ && \
        cp src/redis-server /redis/src/ && \
        rm -rf /tmp/redis \
    ; \
    \
    mkdir -p /redis/6379/data && cp /redis/redis.conf /redis/6379/; \
    \
    # start redis-server :6379
    /redis/src/redis-server /redis/6379/redis.conf --port 6379 --dir /redis/6379/data --daemonize yes; \
    \
    apk del curl make gcc g++