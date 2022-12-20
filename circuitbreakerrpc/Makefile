protoc_os = linux
ifeq ($(shell uname -s),Darwin)
protoc_os = osx
endif

protoc_arch = $(shell uname -m)
ifeq (protoc_arch,aarch64)
protoc_arch = aarch_64
else
protoc_arch = x86_64
endif

protoc-go-docker:
	docker build . -t circuitbreaker-protobuf-builder
	docker run --rm -it -v "$(shell pwd):/app" circuitbreaker-protobuf-builder sh -c 'cd /app && make buf-gen'

protoc-tools:
	wget https://github.com/protocolbuffers/protobuf/releases/download/v3.18.0/protoc-3.18.0-${protoc_os}-${protoc_arch}.zip -O /tmp/protoc.zip
	unzip -d /usr/local /tmp/protoc.zip

protoc-buf:
	curl -sSL  "https://github.com/bufbuild/buf/releases/download/v1.0.0-rc2/buf-$(shell uname -s)-$(shell uname -m).tar.gz" | tar -xvzf - -C /usr/local --strip-components 1

protoc-go-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.27.1
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1
	go install github.com/envoyproxy/protoc-gen-validate@v0.6.1
	go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@v2.5.0

buf-gen:
	buf generate
