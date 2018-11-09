# Required binaries: pv, dockerfiledeps
#
# The latter can be installed with `go get github.com/nyarly/dockerfiledeps`
#
# Usage like
#
# Makefile:
#   REPOSITORY_NAME := project
#   include driver.mk
#
# Put image sources in subdirectories with a Dockerfile

## Registry settings
REGISTRY_HOST ?= docker.otenv.com
REGISTRY_URL := $(REGISTRY_HOST)$(if $(REPOSITORY_NAME),/$(REPOSITORY_NAME))

REPO := $(shell git remote get-url origin)
COMMIT := $(shell git rev-parse HEAD)
VERSION_TAG := $(shell git describe --exact-match 2>/dev/null || echo unversioned)
CLEAN := no
DEFAULT_TAG := dirty

ifeq ($(shell git diff-index --quiet HEAD ; echo $$?),0)
CLEAN := yes
DEFAULT_TAG := latest
else
VERSION_TAG := $(VERSION_TAG)-dirty
endif

TAG := $(or $(TAG),$(DEFAULT_TAG))

TMPDIR ?= /tmp

all: build-all push-all

build-%: .build/%
	@echo -n

push-%: .push/%
	@echo -n

clean:
	rm -rf .build

.tags/base: .pull-once | .tags
	[ "$(shell cat $@)" = "$(TAG)" ] || echo $(TAG) > $@

.tags/version: .pull-once | .tags
	[ "$(shell cat $@)" = "$(VERSION_TAG)" ] || echo $(VERSION_TAG) > $@

## Build rule for all docker stacks
.build/%: docker-deps.mk .tags/base .tags/version | .logs
	cd $* && \
	docker build \
		-t $(REGISTRY_URL)/$*:local \
		-t $(REGISTRY_URL)/$*:$(shell cat .tags/base) \
		-t $(REGISTRY_URL)/$*:$(shell cat .tags/version) \
		--build-arg BUILD_IMAGE_REPO=$(REPO) \
		--build-arg BUILD_IMAGE_REPO=$* \
		--build-arg BUILD_IMAGE_COMMIT=$(COMMIT) \
		--build-arg BUILD_IMAGE_DOCKERFILE=$(shell git ls-tree --full-name --name-only HEAD $<) \
		--build-arg BUILD_IMAGE_CLEAN=$(CLEAN) \
		--build-arg VERSION=$(shell cat .tags/version) \
		$(EXTRA_DOCKER_BUILD_ARGS-$(*)) . \
		> ../.logs/build-$* 2>&1
	touch $@

docker-deps.mk: $(shell find . -type f -name Dockerfile) $(shell which dockerfiledeps)
	grep -q $@ .gitignore || echo $@ >> .gitignore
	dockerfiledeps "$(REGISTRY_URL)" . > docker-deps.mk

include docker-deps.mk

.pull-once:
	@grep -q $@ .gitignore || echo $@ >> .gitignore
	touch .pull-once

.push/%: .tags/base .tags/version .build/% | .push
	docker push $(REGISTRY_URL)/$*:$(shell cat .tags/base) > .logs/push-$* 2>&1
	docker push $(REGISTRY_URL)/$*:$(shell cat .tags/version) >> .logs/push-$* 2>&1
	touch $@

.SECONDARY: .push/%

.build .push .logs .remote .tags:
	@grep -q '^$@$' .gitignore || echo $@ >> .gitignore
	mkdir -p $@

.build/%: DARGS?=

.PHONY: all build-all help clean push-all .pull-once
