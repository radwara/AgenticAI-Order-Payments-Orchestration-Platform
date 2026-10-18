package io.opop.obs;

import io.opentelemetry.exporter.otlp.http.metrics.OtlpHttpMetricExporter;
import io.opentelemetry.sdk.metrics.SdkMeterProvider;
import io.opentelemetry.sdk.metrics.SdkMeterProviderBuilder;
import io.opentelemetry.sdk.metrics.export.PeriodicMetricReader;
import io.opentelemetry.sdk.resources.Resource;
import java.time.Duration;
import org.springframework.beans.factory.ObjectProvider;
import org.springframework.boot.actuate.autoconfigure.health.ConditionalOnEnabledHealthIndicator;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.core.env.Environment;

/** Wires dependency-aware readiness and OTLP metrics export. Tracing is Boot's, see the README. */
@AutoConfiguration
public class ObsAutoConfiguration {

  private static final Duration METRIC_INTERVAL = Duration.ofSeconds(10);

  /** Readiness contributor named {@code dependencies}, included in the readiness group. */
  @Bean
  @ConditionalOnEnabledHealthIndicator("dependencies")
  @ConditionalOnMissingBean(name = "dependenciesHealthIndicator")
  DependencyHealthIndicator dependenciesHealthIndicator(ObjectProvider<DependencyCheck> checks) {
    return new DependencyHealthIndicator(
        checks.orderedStream().toList(), DependencyHealthIndicator.DEFAULT_TIMEOUT);
  }

  /**
   * OTel meter provider; exports over OTLP only when {@code OTEL_EXPORTER_OTLP_ENDPOINT} is a valid
   * URL, otherwise it exports nothing and never fails startup. Closing is bounded by the SDK (10s).
   */
  @Bean(destroyMethod = "close")
  @ConditionalOnMissingBean
  SdkMeterProvider obsMeterProvider(Environment env) {
    SdkMeterProviderBuilder builder =
        SdkMeterProvider.builder()
            .setResource(
                Resource.builder()
                    .put("service.name", env.getProperty("spring.application.name", "unknown"))
                    .build());
    ObsEnvironmentPostProcessor.otlpBase(env)
        .ifPresent(
            base ->
                builder.registerMetricReader(
                    PeriodicMetricReader.builder(
                            OtlpHttpMetricExporter.builder()
                                .setEndpoint(base + "/v1/metrics")
                                .build())
                        .setInterval(METRIC_INTERVAL)
                        .build()));
    return builder.build();
  }
}
