---
id: T-014
title: "[T-014] Transactional outbox for order events"
labels: phase:1, status:needs-approval, lang:java, area:order, size:M
---
## Why
ORD-5: events must never be lost or emitted for rolled-back state. SPEC-02, PRD §8.

## What
Add `outbox(id uuid, aggregate_id, type, payload jsonb, created_at, published_at)`. Order state changes write `OrderCreated`, `OrderPaid`, `OrderCancelled` rows in the same transaction. A relay polls unpublished rows (`FOR UPDATE SKIP LOCKED`) and publishes to Kafka topic `order.events.v1`, keyed by order id, then marks published. At-least-once; event envelope has `event_id`, `type`, `occurred_at`, `schema_version`.

## Constraints
Java 21, Spring Boot 3.x, Maven; Spotless clean; only dependencies named here. Allowed: spring-kafka. Handlers must not call Kafka directly.

## Out of scope
Consumers, schema registry, Debezium/CDC.

## Acceptance criteria (validations)
- [ ] Rolling back the order transaction leaves no outbox row
- [ ] Create then cancel yields events in order for the same key
- [ ] Killing the relay between publish and mark causes a duplicate publish, not a loss (test), and event_id lets a consumer dedupe
- [ ] Two relay instances never publish the same row concurrently
- [ ] Static check (ArchUnit test) fails if any class outside the relay depends on KafkaTemplate

## Files in scope
`services/order/**`, `contracts/events/**`

## Depends on
T-011, T-003
