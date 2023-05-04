
all: build

init:
	# create dirs
	mkdir -p ./build

clear:
	# clear dirs
	rm -rf ./build

build: clear init
	# build
	go build -o ./build/x-ui ./cmd/x-ui/main.go