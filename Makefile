# Integration test stand (see docs/testing/TEST_PLAN.md).
SHELL := /bin/bash
COMPOSE_TEST := docker compose -p r-a-test -f docker-compose.yml -f docker-compose.test.yml
GO_TEST_FLAGS ?= -count=1 -timeout 40m -v
RUN ?= .

.PHONY: test-integration stand-up stand-down stand-logs stand-ps test-integration-run

## Full cycle: build+start the stand, run the suite, tear the stand down.
test-integration: stand-up
	@$(MAKE) --no-print-directory test-integration-run; status=$$?; $(MAKE) --no-print-directory stand-down; exit $$status

## Start (or update) the stand and keep it running (for repeated `make test-integration-run`).
stand-up:
	$(COMPOSE_TEST) up -d --build --remove-orphans

## Run the suite against an already running stand. RUN=TestName narrows it.
## Output: one PASS/FAIL line per test as it finishes (the `=== RUN` noise is dropped).
test-integration-run:
	cd tests && set -o pipefail && go test -tags integration $(GO_TEST_FLAGS) -run '$(RUN)' ./integration/... 2>&1 | grep -vE '^=== (RUN|PAUSE|CONT)'

stand-down:
	$(COMPOSE_TEST) down -v --remove-orphans

stand-logs:
	$(COMPOSE_TEST) logs -f --tail=100 $(SVC)

stand-ps:
	$(COMPOSE_TEST) ps
