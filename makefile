ifndef VERSION
	ifndef RELEASE
	# If we're not publishing a release, set the dev commit hash
		ifndef DEV_TAG_SHA
			COMMIT_HASH :=$(shell git rev-parse --short=7 HEAD)
		else
			COMMIT_HASH :=$(shell echo ${DEV_TAG_SHA} | cut -c 1-7)
		endif
		VERSION := dev-${COMMIT_HASH}
	else
		VERSION := $(shell git describe --tags --abbrev=0)
	endif
endif

.PHONY: build
build:
	go build -o bin/bitaxe-coinbase-checker ./cmd/bitaxe-coinbase-checker

.PHONY: docker docker-push
docker:
	docker build -t adamdecaf/bitaxe-coinbase-checker:${VERSION} .

docker-push:
	docker push adamdecaf/bitaxe-coinbase-checker:${VERSION}

.PHONY: check
check:
ifeq ($(OS),Windows_NT)
	@echo "Skipping checks on Windows, currently unsupported."
else
	@wget -O lint-project.sh https://raw.githubusercontent.com/moov-io/infra/master/go/lint-project.sh
	@chmod +x ./lint-project.sh
	COVER_THRESHOLD=0.0 ./lint-project.sh
endif
