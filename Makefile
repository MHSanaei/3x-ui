APP_NAME = x-ui
DOCKER_IMAGE = my-go-app
BUILD_DIR = build

TARGETARCH ?= amd64
TARGETOS ?= linux
ARCHIVE_NAME = x-ui-$(TARGETOS)-$(TARGETARCH).tar.gz

.PHONY: build clean

build:
	mkdir -p $(BUILD_DIR)
	docker build \
		--build-arg TARGETOS=$(TARGETOS) \
		--build-arg TARGETARCH=$(TARGETARCH) \
		--target export-stage \
		-o $(BUILD_DIR) .

clean:
	rm -rf $(BUILD_DIR)
	docker rmi $(DOCKER_IMAGE)