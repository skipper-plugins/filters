SOURCES            = $(shell find ./ -type f -name '*.go' | grep -vE './(build|vendor)/' )
FILTERS           ?= geoip noop
PLUGINS            = $(shell for t in $(FILTERS); do echo build/filter_$$t.so; done )
CURRENT_VERSION    = $(shell git tag | sort -V | tail -n1)
VERSION           ?= $(CURRENT_VERSION)
COMMIT_HASH        = $(shell git rev-parse --short HEAD)
XGOPATH            = $(HOME)/go

default: plugins

plugins: deps $(PLUGINS) checks

geoipdb: GeoLite2-Country.mmdb

GeoLite2-Country.mmdb: GeoLite2-Country.tar.gz
	tar xvzf GeoLite2-Country.tar.gz --wildcards --strip-components=1 '*/GeoLite2-Country.mmdb'


GeoLite2-Country.tar.gz:
	wget -O GeoLite2-Country.tar.gz http://geolite.maxmind.com/download/geoip/database/GeoLite2-Country.tar.gz

deps: 
	go get -t github.com/zalando/skipper
	# glide update
	glide install

checks: vet fmt tests

tests: geoipdb
	go test -run LoadPlugin

vet: $(SOURCES)
	go vet $(shell for t in $(FILTERS); do echo ./$$t/...; done )

fmt: $(SOURCES)
	@if [ "$$(gofmt -d $(SOURCES))" != "" ]; then false; else true; fi

test:
	go test -v

$(PLUGINS): $(SOURCES)
	mkdir -p build/
	MODULE=$(shell basename $@ .so | sed -e 's/filter_//' ); \
		   go build -buildmode=plugin -o $@ $$MODULE/$$MODULE.go

clean:
	rm -f build/*.so
realclean: clean
	rm -f GeoLite2-Country.mmdb GeoLite2-Country.tar.gz
