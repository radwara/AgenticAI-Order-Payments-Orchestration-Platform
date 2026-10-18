# Root Makefile: build / test / lint every module under services/* and libs/*.
# A module participates by providing its own Makefile with the matching target;
# modules without a Makefile (or without that target) are skipped.
# All modules run even if one fails; make then exits non-zero.

MODULE_DIRS := services libs

.PHONY: all build test lint help dev-up dev-down

all: build

help:
	@echo "Targets: build test lint (each iterates services/* and libs/*); dev-up dev-down (local stack)"

build test lint:
	@fail=0; \
	for dir in $(MODULE_DIRS); do \
	  for mod in $$dir/*/; do \
	    [ -f "$${mod}Makefile" ] || continue; \
	    out=$$($(MAKE) -n -C "$$mod" $@ 2>&1); rc=$$?; \
	    if [ $$rc -ne 0 ] && printf '%s\n' "$$out" | grep -Eq "No rule to make target .$@."; then \
	      echo "skip $${mod%/}: no '$@' target"; continue; \
	    fi; \
	    echo "==> $@ $${mod%/}"; \
	    $(MAKE) --no-print-directory -C "$$mod" $@ || { echo "FAILED: $@ $${mod%/}"; fail=1; }; \
	  done; \
	done; \
	exit $$fail

# Local dev stack (Postgres, Kafka, Redis). Fake local-only creds in deploy/local/.env.example.
LOCAL_COMPOSE := docker compose -f deploy/local/docker-compose.yml --env-file deploy/local/.env.example

dev-up:
	$(LOCAL_COMPOSE) up -d --wait --remove-orphans
	deploy/local/verify.sh

dev-down:
	$(LOCAL_COMPOSE) down --remove-orphans --volumes
