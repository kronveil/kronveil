.PHONY: dev-setup build test lint docker-build helm-lint

dev-setup:
	@echo "Setting up Kronveil development environment..."
	go mod download
	pip install -r intelligence/requirements.txt
	cd dashboard && npm install

build:
	go build ./...

test:
	go test ./... -v -race
	python -m pytest intelligence/ -v

lint:
	golangci-lint run
	cd dashboard && npm run lint

docker-build:
	docker build -t kronveil/agent:latest -f deploy/Dockerfile.agent .
	docker build -t kronveil/dashboard:latest -f deploy/Dockerfile.dashboard .

helm-lint:
	helm lint helm/kronveil/
