FROM golang:1.16.6-alpine

ARG GolangciVersion=v1.41.1
ARG RedisVersion=6.2.5

RUN set -eux; \
    \
    apk add --no-cache curl make gcc g++ musl-dev; \
    \
    # install golangci-lint
    curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(go env GOPATH)/bin $GolangciVersion; \
    \
    # install redis-server
    mkdir -p /redis/src && mkdir -p /tmp/redis; \
    wget -qO- https://download.redis.io/releases/redis-$RedisVersion.tar.gz | tar xvz --strip-components=1 -C /tmp/redis; \
    cd /tmp/redis && \
        make all && \
        cp redis.conf /redis/redis.conf && \
        cp src/redis-server /redis/src/redis-server && \
        rm -rf /tmp/redis \
    ; \
    \
    mkdir -p /opt/6379/data; \
    cp /redis/redis.conf /opt/6379/; \
    \
    apk del curl make g++

CMD ["/redis/src/redis-server", "/opt/6379/redis.conf", "--port", "6379", "--dir", "/opt/6379/data"]
