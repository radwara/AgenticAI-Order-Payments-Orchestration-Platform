.PHONY: build test lint

build:
	mvn -B -DskipTests package

test:
	mvn -B verify

lint:
	mvn -B spotless:check
