package io.opop.obs;

import io.opentelemetry.exporter.otlp.http.metrics.OtlpHttpMetricExporter;
import io.opentelemetry.sdk.metrics.SdkMeterProvider;
import io.opentelemetry.sdk.metrics.SdkMeterProviderBuilder;
import io.opentelemetry.sdk.metrics.export.PeriodicMetricReader;
import io.opentelemetry.sdk.resources.Resource;
import java.time.Duration;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.core.env.Environment;

/** Wires OTLP metrics export. Tracing is Spring Boot's, configured by the post-processor. */
@AutoConfiguration
public class ObsAutoConfiguration {

  private static final Duration METRIC_INTERVAL = Duration.ofSeconds(10);

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
