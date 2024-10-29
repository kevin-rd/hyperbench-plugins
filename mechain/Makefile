# binary_name
BINARY_NAME=mechain.so

## build: build the binary with pre-packed static resource
build:
	@export GO111MODULE=on
	@export GOPROXY=https://goproxy.cn,direct
	@go mod download
	@go build -v -buildmode=plugin -trimpath -o $(BINARY_NAME)
