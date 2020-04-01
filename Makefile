GOOS=$(shell go env GOOS)

.DEFAULT_GOAL := all

.PHONY: all
all: build docker-image

.PHONY: build-native 
build-native: install-tools
	go generate -mod=vendor main.go
	GO111MODULE=on CGO_ENABLED=0 go build -mod=vendor -o ./bin/hotrod-$(GOOS) main.go

.PHONY: build
build:
	docker run -it --rm -v $(PWD):/home/circleci/project cimg/go:1.13 make build-native

.PHONY: docker-image
docker-image:
	docker build -t hotrod-rum .

.PHONY: install-tools
install-tools:
	go install -mod=vendor github.com/mjibson/esc