# The Makefile uses a dockerized go enviroment
# See: https://www.iron.io/the-easiest-way-to-develop-with-go%E2%80%8A-%E2%80%8Aintroducing-a-docker-based-go-tool/

GO=golang:1.6
GO=mpfmedical/golang-glide

PUSH_REGISTRY=registry.kramergroup.science
APP_TAG=util/peerfinder:latest

SRCDIR=$(shell pwd | sed 's|src.*$$|src|g')
WRKPATH=$(shell pwd | sed 's|^.*src/||')

PKGDIR=$(shell pwd)/pkg
BINDIR=$(shell pwd)/bin

SOURCES := $(shell find . -not -path "./vendor/*" -not -path "./.glide/*" -name '*.go')

.PHONY: container
container: $(BINDIR)/peerfinder Dockerfile
	docker build -t $(PUSH_REGISTRY)/$(APP_TAG) .

$(BINDIR)/peerfinder: $(SOURCES) $(PKGDIR) $(BINDIR) vendor
	docker run --rm -v $(SRCDIR):/go/src \
									-v $(PKGDIR):/go/pkg \
									-v $(BINDIR):/go/bin \
									-w /go/src/$(WRKPATH) \
									$(GO) sh -c 'CGO_ENABLED=0 go install -a --installsuffix cgo --ldflags="-s"'

vendor:
	docker run --rm -v $(SRCDIR):/go/src \
									-w /go/src/$(WRKPATH) $(GO) glide up

.PHONY: edit
edit:
	GOPATH=$(shell pwd | sed 's|src.*$$||g'):$(PWD)/vendor atom .

.PHONY: run
run: $(BINDIR)/peerfinder
	docker run --rm -v $(BINDIR):/app -w /app \
									-v $(HOME)/.kube:/kube \
									-p 8080:8080 \
									centos ./peerfinder -kubeconfig=/kube/config

.PHONY: package
package: $(BINDIR)/peerfinder
	docker run --rm -v $(BINDIR):/app -w /app \
									-v $(SRCDIR):/go/src \
									-v $(PKGDIR):/go/pkg \
									-v /var/run/docker.sock:/var/run/docker.sock \
									treeder/go image $(PUSH_REGISTRY)/$(APP_TAG)

$(PKGDIR) $(BINDIR):
	mkdir -p $@

.PHONY: push
push: container
	docker push $(PUSH_REGISTRY)/$(APP_TAG)

clean:
	rm -rf vendor
	rm -rf peerfinder
	rm -rf $(PKGDIR)
	rm -rf $(BINDIR)
