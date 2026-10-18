# payment module: Java 21 + Maven. Spotless and JaCoCo are configured in pom.xml.
.PHONY: build test lint

build:
	mvn -B -DskipTests package

test:
	mvn -B verify

lint:
	mvn -B spotless:check
