# GRTE (go-redis-test-env)

GRTE is a tool that can run go-redis unit tests locally.  
So you don’t have to worry about the virtual environment of unit tests anymore.  
It is only compatible with the latest version of go-redis master branch.  

## Install

1. First you need to [install a Docker](https://docs.docker.com/get-docker).  

2. You can use the easiest way to install grte.  
```
> git clone https://github.com/monkey92t/grte.git
> cd grte
> go build -o $GOPATH/bin/grte
```

## Quick Start

```
> pwd
/home/work/go-redis/redis
> grte go test ./...
......
......
......
```

If you see the following output, it means the test has been executed:  
```
monkey92t@iMac redis % grte go test ./...
[go-redis]  🐳 Prepare docker image ==> goredis/grte:env
[go-redis]  🐳 Use image cache, ID ==> sha256:6a32c20dcf1527edde9172a2787dad9685c308e26c546d8344a1f8ecd07c3fa0
[go-redis]  🐳 Create docker container...
[go-redis]  🐳 ContainerID: eec5155412aa0a8d6ed2aa9810770ebf9343840f91fee216dd9cf1cb10c144e4
[go-redis]  🐳 WorkDir: /Users/monkey/redis
[go-redis]  🐳 Command: [go test ./...]
ok      github.com/go-redis/redis/v8    98.691s
ok      github.com/go-redis/redis/v8/internal   0.049s
ok      github.com/go-redis/redis/v8/internal/hashtag   0.097s
ok      github.com/go-redis/redis/v8/internal/hscan     0.060s
ok      github.com/go-redis/redis/v8/internal/pool      1.143s
ok      github.com/go-redis/redis/v8/internal/proto     0.026s
?       github.com/go-redis/redis/v8/internal/rand      [no test files]
?       github.com/go-redis/redis/v8/internal/util      [no test files]
[go-redis]  ✅  Success!
```

You can execute `golangci-lint run` command:  

```
....
....
[go-redis]  🐳 WorkDir: /Users/monkey/redis
[go-redis]  🐳 Command: [golangci-lint run]
WARN [runner] The linter 'interfacer' is deprecated (since v1.38.0) due to: The repository of the linter has been archived by the owner.  
WARN [runner] The linter 'scopelint' is deprecated (since v1.39.0) due to: The repository of the linter has been deprecated by the owner.  Replaced by exportloopref. 
WARN [runner] The linter 'golint' is deprecated (since v1.41.0) due to: The repository of the linter has been archived by the owner.  Replaced by revive. 
[go-redis]  ✅  Success!
```

Its working directory depends on where you are currently in `go-redis/redis`:  
```
monkey@iMac pool % pwd
/Users/monkey/redis/internal/pool
monkey92t@iMac pool % grte go test ./...
[go-redis]  🐳 Prepare docker image ==> goredis/grte:env
[go-redis]  🐳 Use image cache, ID ==> sha256:b69aee248b1d3ebd47a1115773ffa22dccea3cff5433d140dbad092e9326a977
[go-redis]  🐳 Create docker container...
[go-redis]  🐳 ContainerID: 5d7fd7fc66cf811df7533c68e1b4e12d0e35ed199a6a072a7b2b668b2022c70b
[go-redis]  🐳 WorkDir: /Users/monkey/redis/internal/pool
[go-redis]  🐳 Command: [go test ./...]
ok      github.com/go-redis/redis/v8/internal/pool      0.953s
[go-redis]  ✅  Success!
monkey@iMac pool % 

```

For complex commands, you may need to use `"` to handle:  
```
> grte "go vet go test ./... && go vet"
```

If you want to use the environment variables in the virtual environment, remember to use "\":  
```
> grte echo \$PATH
```

WARN: The original intention of GRTE is only to quickly execute go-redis unit tests, not to execute overly complex commands.  

## Configuration File

GRTE will read the go-redis/grte.yaml file.  
You can create a `~/.grte.yaml` file (please note that it is `~/.grte.yaml`) to override the options of go-redis/grte.yaml

```
// The minimum version number of grte, if it is lower than it, you need to upgrade grte.
MinVersionNumber: 100

// The Docker image version used, if there is a local cache, the cache is preferred,
// otherwise it is downloaded from `https://docker.io`
Image: goredis/grte:env

// Environment variables set in the container.
ContainerEnv:
  GOPROXY: https://goproxy.io
  ENV1: Value1
  ENV2: Value2
```

## WARN

GRTE will automatically use `/temp/go-redis-test-env-gopath` to cache `go mod`.
  
  

Thanks!  
END...  