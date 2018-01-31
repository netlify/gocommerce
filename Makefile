.PONY: all build deps image lint test

help: ## Show this help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

all: lint test build ## Run the tests and build the binary.

os = darwin
arch = amd64

build: test
	@echo "Making gocommerce for $(os)/$(arch)"
	GOOS=$(os) GOARCH=$(arch) go build -ldflags "-X github.com/netlify/gocommerce/cmd.Version=`git rev-parse HEAD`"

build_linux: override os=linux
build_linux: build

package: build
	tar -czf gocommerce-$(os)-$(arch).tar.gz gocommerce

package_linux: override os=linux
package_linux: package

release: ## Upload release to GitHub releases.
	mkdir -p builds/darwin-${TAG}
	GOOS=darwin GOARCH=$(arch) go build -ldflags "-X github.com/netlify/gocommerce/cmd.Version=`git rev-parse HEAD`" -o builds/darwin-${TAG}/gocommerce
	mkdir -p builds/linux-${TAG}
	GOOS=linux GOARCH=$(arch) go build -ldflags "-X github.com/netlify/gocommerce/cmd.Version=`git rev-parse HEAD`" -o builds/linux-${TAG}/gocommerce
	mkdir -p builds/windows-${TAG}
	GOOS=windows GOARCH=$(arch) go build -ldflags "-X github.com/netlify/gocommerce/cmd.Version=`git rev-parse HEAD`" -o builds/windows-${TAG}/gocommerce.exe
	@rm -rf releases/${TAG}
	mkdir -p releases/${TAG}
	tar -czf releases/${TAG}/gocommerce-darwin-$(arch)-${TAG}.tar.gz -C builds/darwin-${TAG} gocommerce
	tar -czf releases/${TAG}/gocommerce-linux-$(arch)-${TAG}.tar.gz -C builds/linux-${TAG} gocommerce
	zip -j releases/${TAG}/gocommerce-windows-$(arch)-${TAG}.zip builds/windows-${TAG}/gocommerce.exe
	@hub release create -a releases/${TAG}/gocommerce-darwin-$(arch)-${TAG}.tar.gz -a releases/${TAG}/gocommerce-linux-$(arch)-${TAG}.tar.gz -a releases/${TAG}/gocommerce-windows-$(arch)-${TAG}.zip v${TAG}


deps: ## Install dependencies.
	@go get -u github.com/golang/lint/golint
	@go get -u github.com/Masterminds/glide && glide install

image: ## Build the Docker image.
	docker build .

lint: ## Lint the code
	golint `go list ./... | grep -v /vendor/`

test: ## Run tests.
	go test -v `go list ./... | grep -v /vendor/`
