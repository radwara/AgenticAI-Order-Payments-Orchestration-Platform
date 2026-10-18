# Root Makefile: build / test / lint every module under services/* and libs/*.
# A module participates by providing its own Makefile with the matching target;
# modules without a Makefile (or without that target) are skipped.
# All modules run even if one fails; make then exits non-zero.

MODULE_DIRS := services libs

.PHONY: all build test lint help

all: build

help:
	@echo "Targets: build test lint (each iterates services/* and libs/*)"

build test lint:
	@fail=0; \
	for dir in $(MODULE_DIRS); do \
	  for mod in $$dir/*/; do \
	    [ -f "$${mod}Makefile" ] || continue; \
	    if ! $(MAKE) -n -C "$$mod" $@ >/dev/null 2>&1; then \
	      echo "skip $${mod%/}: no '$@' target"; continue; \
	    fi; \
	    echo "==> $@ $${mod%/}"; \
	    $(MAKE) --no-print-directory -C "$$mod" $@ || { echo "FAILED: $@ $${mod%/}"; fail=1; }; \
	  done; \
	done; \
	exit $$fail
