.PHONY: build test lint docker push deploy clean run

BINARY := bin/scheduler
IMAGE  := ghcr.io/your-org/custom-scheduler
TAG    := latest

build:
	CGO_ENABLED=0 go build -ldflags="-w -s" -o $(BINARY) ./cmd/scheduler

run: build
	./$(BINARY) --policy=bin-packing --log-level=debug

test:
	go test ./... -v -race -cover

lint:
	go vet ./...
	gofmt -l .

docker:
	docker build -t $(IMAGE):$(TAG) .

push: docker
	docker push $(IMAGE):$(TAG)

deploy:
	kubectl apply -f deploy/scheduler.yaml

undeploy:
	kubectl delete -f deploy/scheduler.yaml

example-pods:
	kubectl apply -f deploy/example-pods.yaml

clean:
	rm -rf bin/
