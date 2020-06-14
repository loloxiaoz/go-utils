IMPORT_PATH:=github.com:loloxiaoz/go-utils

# params
TIME			:= $(shell date '+%Y-%m-%d %H:%M:%S')
BRANCH			:= $(shell git symbolic-ref --short -q HEAD)
SERIAL			:= $(shell git rev-parse --short HEAD)
VERSION			:= $(shell git rev-parse --abbrev-ref HEAD)
GOVERSION		:= $(shell go version)
COMMITID		:= $(shell git log  -1 --pretty=format:"%h")
COMMITDATE		:= $(shell git show -s --format=%ci)

# Go parameters
GOCMD=go
GORUN=$(GOCMD) run
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BUILDPATH=build
BINPATH=$(BUILDPATH)
PROJECT=go-utils

all: test 

.PHONY: test   
test:
	go test ./... -parallel=1  -timeout=30m -coverprofile=./coverage.out -v  
