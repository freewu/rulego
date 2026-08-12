# rulego 常用命令

.PHONY: build run test test-frontend examples clean

build:
	go build -o rulego ./cmd/rulego

run: build
	./rulego -c config.example.yaml

test:
	go test ./...

test-frontend:
	cd web && npm test

examples:
	cd web && npm run gen-examples

clean:
	rm -f rulego
	rm -rf data
