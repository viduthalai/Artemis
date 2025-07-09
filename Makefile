.PHONY: clean build package deploy build-cli

# Go binary targets
build:
	mkdir -p _bin
	GOOS=linux GOARCH=amd64 go build -o _bin/bootstrap main.go

build-cli:
	@echo "Building Artemis CLI..."
	@mkdir -p _bin
	go build -o _bin/artemis ./cmd

package: clean build
	zip _bin/function.zip _bin/bootstrap

deploy: package
	aws lambda update-function-code \
		--function-name stbn-trading-bot \
		--zip-file fileb://_bin/function.zip \
		--profile personal \
		--region us-east-2

clean:
	rm -rf _bin/

# Example usage:
# make deploy  # for deploying Go function
# make build-cli  # for building the CLI 