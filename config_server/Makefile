IMAGE_NAME:=refract-config-server
IMAGE_TAG:=$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo local)

.DEFAULT_GOAL := image

image:
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
build:
	go build -o config_server .
all: image
