VERSION            ?= $(shell git describe --exact-match --tags $(git log -n1 --pretty='%h') || git describe --dirty --tags)
DOCKER_IMAGE_NAME  ?= registry.twiket.com/r2s

all: build

release:
	@echo "========================================================="
	@echo "RELEASE: $(DOCKER_IMAGE_NAME):$(VERSION)"
	@echo "========================================================="
	make build
	make push

build:
	@echo ">> building docker image"
	@docker build -t "$(DOCKER_IMAGE_NAME):$(VERSION)" .

push:
	@echo ">> pushing release to $(DOCKER_IMAGE_NAME):$(VERSION)"
	@docker push "$(DOCKER_IMAGE_NAME):$(VERSION)"

.PHONY: docker-build docker-release
