.PHONY: build test lint

build:
	mvn -B -q -DskipTests package

test:
	mvn -B verify

lint:
	mvn -B spotless:check
