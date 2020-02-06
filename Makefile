TS=$(shell date -u +"%F_%T")
TAG=$(shell git tag | sort --version-sort | tail -1)
COMMIT=$(shell git log --oneline | head -1)
VERSION=$(firstword $(COMMIT))
TARGET=121proxy.go
BINPROXY=$(shell basename "$(PWD)")
BINSERVER=echo_server
COVERAGE=coverage.out
PROXYPKG=github.com/z0rr0/121proxy/proxy
TESTCFG=/tmp/121cfg.json

all: clean server build

lint:
	go vet 121proxy.go
	go vet $(PROXYPKG)
	go vet server/server.go
	golint 121proxy.go
	golint $(PROXYPKG)
	golint server/server.go

test: lint
	@-cp config.json $(TESTCFG)
	go test -race -v -cover -coverprofile=$(COVERAGE) -trace trace.out $(PROXYPKG)

build: test
	go build -o $(BINPROXY) -ldflags '-X main.Version=$(TAG) -X main.Revision=git:$(VERSION) -X main.BuildDate=$(TS)' $(TARGET)

params:
	@echo "  >  $(NAME) -TS $(TS) - $(TAG) - $(VERSION)"

server: lint
	go build -o $(BINSERVER) server/server.go

clean:
	rm -vf $(BINPROXY) $(BINSERVER) $(COVERAGE)
