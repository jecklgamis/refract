IMAGE_NAME:=refract
IMAGE_TAG:=$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo local)

.DEFAULT_GOAL := all

# rendered/ is gitignored and baked into the image (Dockerfile COPY), so it
# must be generated from config/values.yaml first - same step CI runs.
# Local files only: render never pushes anywhere.
render: build-refractctl
	./refractctl/refractctl render
image: render
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .
# config_server is its own module with its own Makefile and image.
config-server-image:
	$(MAKE) -C config_server image
# refractctl is a self-contained Go module (own go.mod, own Makefile) -
# a plain static binary, installable anywhere with no runtime dependency.
build-refractctl:
	$(MAKE) -C refractctl build
# Builds and `go install`s refractctl into $(go env GOPATH)/bin (put that on
# your PATH to run it as plain `refractctl`).
install-cli:
	$(MAKE) -C refractctl install
# Cross-compiles refractctl for macOS (arm64/amd64) and Linux (amd64/arm64)
# into refractctl/dist/ - hand these to teammates directly, no Go toolchain
# needed on their end.
build-refractctl-all:
	$(MAKE) -C refractctl build-all
# Builds the gateway image with a values.yaml covering every add-backend
# feature (TLS, HTTP/2, host-rewrite, domain/path-prefix routing, gzip
# compression), boots it, and checks Envoy actually accepted the config
# (not just that it compiled and marshaled) plus that the features behave
# as configured. Run this after any change to
# refractctl/internal/{render,envoyconfig}, config/envoy.yaml, or the
# Dockerfile. See scripts/integration-test.sh for why this exists.
integration-test:
	./scripts/integration-test.sh
all: image config-server-image build-refractctl
