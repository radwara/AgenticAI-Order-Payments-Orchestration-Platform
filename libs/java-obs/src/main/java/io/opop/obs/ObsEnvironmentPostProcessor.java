package io.opop.obs;

import java.net.URI;
import java.util.Map;
import java.util.Optional;
import org.apache.commons.logging.Log;
import org.springframework.boot.SpringApplication;
import org.springframework.boot.env.EnvironmentPostProcessor;
import org.springframework.boot.logging.DeferredLogFactory;
import org.springframework.core.env.ConfigurableEnvironment;
import org.springframework.core.env.Environment;
import org.springframework.core.env.MapPropertySource;

/**
 * Adds lowest-precedence defaults (application config always wins): OTLP trace export driven by
 * {@code OTEL_EXPORTER_OTLP_ENDPOINT}, health probes with dependency checks in readiness, always
 * sampled traces, and JSON console logs with trace_id.
 */
public class ObsEnvironmentPostProcessor implements EnvironmentPostProcessor {

  /** Standard env var naming the OTLP/HTTP collector base URL. */
  public static final String ENDPOINT_ENV = "OTEL_EXPORTER_OTLP_ENDPOINT";

  private final Log log;

  public ObsEnvironmentPostProcessor(DeferredLogFactory logFactory) {
    this.log = logFactory.getLog(ObsEnvironmentPostProcessor.class);
  }

  @Override
  public void postProcessEnvironment(ConfigurableEnvironment env, SpringApplication application) {
    Optional<String> base = otlpBase(env);
    if (base.isEmpty()) {
      boolean unset = env.getProperty(ENDPOINT_ENV, "").isBlank();
      log.info(
          ENDPOINT_ENV
              + (unset ? " not set" : " is not a valid http(s) URL")
              + "; telemetry is not exported");
    }
    env.getPropertySources()
        .addLast(
            new MapPropertySource(
                "java-obs-defaults",
                Map.of(
                    "management.endpoint.health.probes.enabled",
                    "true",
                    "management.endpoint.health.group.readiness.include",
                    "readinessState,dependencies",
                    "management.tracing.sampling.probability",
                    "1.0",
                    "logging.structured.format.console",
                    TraceJsonLogFormatter.class.getName(),
                    "management.otlp.tracing.export.enabled",
                    String.valueOf(base.isPresent()),
                    "management.otlp.tracing.endpoint",
                    base.map(b -> b + "/v1/traces").orElse(""))));
  }

  /** The collector base URL without trailing slash, or empty if unset or not an http(s) URL. */
  static Optional<String> otlpBase(Environment env) {
    String raw = env.getProperty(ENDPOINT_ENV, "").trim();
    try {
      URI uri = URI.create(raw);
      boolean http = "http".equals(uri.getScheme()) || "https".equals(uri.getScheme());
      return http && uri.getHost() != null
          ? Optional.of(raw.replaceAll("/+$", ""))
          : Optional.empty();
    } catch (IllegalArgumentException e) {
      return Optional.empty();
    }
  }
}
