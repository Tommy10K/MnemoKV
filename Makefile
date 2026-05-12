GO        ?= go
PKGS      := ./...
BIN_DIR   := bin
NODE_BIN  := $(BIN_DIR)/mnemokv-node
WORKLOAD_BIN := $(BIN_DIR)/mnemokv-workload
ADMIN_BIN := $(BIN_DIR)/mnemokv-adminctl

.PHONY: all build test race vet fmt run clean tidy bench smoke

all: build

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(NODE_BIN) ./cmd/node
	$(GO) build -o $(WORKLOAD_BIN) ./cmd/workload
	$(GO) build -o $(ADMIN_BIN) ./cmd/adminctl

test:
	$(GO) test $(PKGS)

race:
	$(GO) test -race $(PKGS)

vet:
	$(GO) vet $(PKGS)

fmt:
	$(GO) fmt $(PKGS)

bench:
	$(GO) test -bench=. -benchmem -run=^$$ $(PKGS)

run: build
	./$(NODE_BIN) --config configs/standalone.yaml

smoke:
	./scripts/smoke-test.sh

tidy:
	$(GO) mod tidy

clean:
	rm -rf $(BIN_DIR)
