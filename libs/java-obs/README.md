# java-obs

Spring Boot 3 auto-configuration starter for observability (T-025). Java counterpart of `libs/go-obs`. Add the dependency; no code is required.

- **Tracing and propagation.** Spring Boot's Micrometer Observation/Tracing with the OTel bridge creates server spans and propagates W3C `traceparent` on HTTP servers and on clients built from Boot's auto-configured builders (tested with `RestClient.Builder`). Traces are always sampled by default (`management.tracing.sampling.probability=1.0`); override it in application config. gRPC is not covered: it needs a gRPC dependency the ticket's Constraints do not name.
- **OTLP export from env.** `OTEL_EXPORTER_OTLP_ENDPOINT` (OTLP/HTTP base URL, e.g. `http://collector:4318`) enables trace export to `<base>/v1/traces` and metric export (OTel SDK `SdkMeterProvider` bean, 10s interval) to `<base>/v1/metrics`. Unset, blank, or not an http(s) URL: nothing is exported and startup never fails. An unreachable collector does not fail startup either. Other `OTEL_*` variables are not consulted. Beans are closed on shutdown, bounded by the OTel SDK (10s).
- **JSON logs.** Console logging uses a JSON formatter (`TraceJsonLogFormatter`): one object per line with `timestamp`, `level`, `logger`, `message`, and `trace_id`/`span_id` (from the MDC) when inside a span, plus `stack_trace`. Override with `logging.structured.format.console`.
- **Health.** Probes are enabled: `/actuator/health/liveness` (always UP while the process serves) and `/actuator/health/readiness` (503 when DOWN). Readiness includes every `DependencyCheck` bean through a `dependencies` indicator: checks run concurrently, the probe is bounded by 2s even if a check ignores interruption, a check still running from an earlier probe is not started again (reported DOWN), and the response lists only check names with UP/DOWN. Error text goes to the log only. Any other `HealthIndicator` can be added to the group via `management.endpoint.health.group.readiness.include`.

All defaults sit at the lowest property precedence, so application config wins.

Run `make test` and `make lint` in this directory (`mvn -B spotless:check verify`). The JaCoCo report is `target/site/jacoco/jacoco.xml`.
