ifeq ($(OS),Windows_NT)
SHELL := sh.exe
else
SHELL := /bin/sh
endif
export GOTOOLCHAIN ?= local
GO ?= go
PYTHON ?= python3
.PHONY: bootstrap build fmt fmt-check lint typecheck unit integration acceptance test race verify-seed verify demo fuzz mutation benchmark clean-verify public-audit integrity security security-online teardown target system differential campaign real-lab
bootstrap:
	$(PYTHON) scripts/bootstrap.py
build:
	$(PYTHON) scripts/build.py
fmt:
	gofmt -w cmd internal tests
fmt-check:
	@files=$$(gofmt -l cmd internal tests) || exit $$?; test -z "$$files" || (printf "%s\n" "$$files"; exit 1)
lint:
	$(GO) vet ./...
	$(PYTHON) -m compileall -q scripts
typecheck:
	$(GO) test -run '^$$' ./...
unit:
	$(GO) test -count=1 ./internal/... ./cmd/...
integration:
	$(GO) test -count=1 ./tests/integration
acceptance:
	$(GO) test -count=1 ./tests/acceptance_locked
integrity:
	$(PYTHON) scripts/check_integrity.py
public-audit:
	$(PYTHON) scripts/public_audit.py
test:
	$(GO) test -count=1 ./...
	$(PYTHON) -m unittest discover -s scripts -p 'test_*.py' -v
race:
	$(GO) test -race -count=1 ./...
verify-seed: bootstrap integrity fmt-check build lint typecheck test race public-audit
	@echo 'Implemented source checks passed. Full release evidence and independent review are a separate gate.'
verify: verify-seed
	$(PYTHON) scripts/full_gate.py
	$(PYTHON) scripts/supported_toolchain.py
	$(MAKE) system differential campaign real-lab security-online

demo: build
	$(PYTHON) scripts/demo.py
fuzz:
	GOMAXPROCS=2 $(GO) test ./internal/history -run '^$$' -fuzz '^FuzzHistory$$' -fuzztime=3s -parallel=2
	GOMAXPROCS=2 $(GO) test ./internal/sim -run '^$$' -fuzz '^FuzzReplay$$' -fuzztime=3s -parallel=2
	GOMAXPROCS=2 $(GO) test ./tests/security -run '^$$' -fuzz '^FuzzScenarioSchema$$' -fuzztime=3s -parallel=2
	GOMAXPROCS=2 $(GO) test ./tests/security -run '^$$' -fuzz '^FuzzTargetTraceReplay$$' -fuzztime=3s -parallel=2
mutation:
	$(PYTHON) scripts/mutation.py
	$(PYTHON) scripts/target_mutation.py
benchmark:
	$(GO) test ./internal/history ./internal/sim ./tests/system -run '^$$' -bench . -benchmem -benchtime=100ms -cpu=4 -count=7
clean-verify:
	$(PYTHON) scripts/clean_verify.py
security:
	$(GO) test -count=1 ./tests/security ./internal/report ./internal/strictjson
security-online:
	$(PYTHON) scripts/security_online.py
teardown:
	$(PYTHON) scripts/teardown.py
# These suites execute the instrumented target; no mock integration is credited.
target system:
	$(GO) test -count=1 ./tests/system

differential:
	$(GO) test -count=1 ./tests/differential
campaign: build
	$(PYTHON) scripts/campaign.py
real-lab: build
	$(PYTHON) scripts/real_lab.py
