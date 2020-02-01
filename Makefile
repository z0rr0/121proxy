NAME=$(shell basename "$(PWD)")
TS=$(shell date -u +\"%F_%T\")
TAG=$(shell git tag | sort --version-sort | tail -1)
COMMIT=$(shell git log --oneline | head -1)
VERSION=$(firstword $(COMMIT))
TARGET="121proxy.go"

build:
	go build -ldflags '-X main.Version=$(TAG) -X main.Revision=git:$(VERSION) -X main.BuildDate=$(TS)' $(TARGET)

params:
	@echo "  >  $(NAME) -TS $(TS) - $(TAG) - $(VERSION)"
