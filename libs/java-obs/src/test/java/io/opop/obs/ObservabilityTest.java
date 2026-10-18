package io.opop.obs;

import static org.assertj.core.api.Assertions.assertThat;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.micrometer.tracing.Tracer;
import io.opentelemetry.sdk.metrics.SdkMeterProvider;
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeUnit;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;
import org.springframework.boot.SpringBootConfiguration;
import org.springframework.boot.WebApplicationType;
import org.springframework.boot.autoconfigure.EnableAutoConfiguration;
import org.springframework.boot.builder.SpringApplicationBuilder;
import org.springframework.boot.test.system.CapturedOutput;
import org.springframework.boot.test.system.OutputCaptureExtension;
import org.springframework.context.ConfigurableApplicationContext;
import org.springframework.context.annotation.Import;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.client.RestClient;

@ExtendWith(OutputCaptureExtension.class)
class ObservabilityTest {

  private static final ObjectMapper JSON = new ObjectMapper();
  private static final HttpClient HTTP = HttpClient.newHttpClient();

  @SpringBootConfiguration
  @EnableAutoConfiguration
  @Import(Hop.class)
  static class App {}

  @RestController
  static class Hop {
    private static final org.slf4j.Logger LOG = org.slf4j.LoggerFactory.getLogger(Hop.class);
    private final Tracer tracer;
    private final RestClient.Builder client;

    Hop(Tracer tracer, RestClient.Builder client) {
      this.tracer = tracer;
      this.client = client;
    }

    @GetMapping("/ping")
    String ping() {
      LOG.info("handling ping");
      return tracer.currentSpan().context().traceId();
    }

    @GetMapping("/call")
    Map<String, String> call(@RequestParam("port") int port) {
      String own = tracer.currentSpan().context().traceId();
      String remote =
          client
              .build()
              .get()
              .uri("http://127.0.0.1:" + port + "/ping")
              .retrieve()
              .body(String.class);
      return Map.of("own", own, "remote", remote);
    }
  }

  private static ConfigurableApplicationContext start(String name, String endpoint) {
    return new SpringApplicationBuilder(App.class)
        .web(WebApplicationType.SERVLET)
        .bannerMode(org.springframework.boot.Banner.Mode.OFF)
        .run(
            "--server.port=0",
            "--spring.application.name=" + name,
            "--OTEL_EXPORTER_OTLP_ENDPOINT=" + endpoint);
  }

  private static String base(ConfigurableApplicationContext ctx) {
    return "http://127.0.0.1:" + ctx.getEnvironment().getProperty("local.server.port");
  }

  private static HttpResponse<String> get(String url) throws Exception {
    return HTTP.send(
        HttpRequest.newBuilder(URI.create(url)).GET().build(),
        HttpResponse.BodyHandlers.ofString());
  }

  private static boolean contains(byte[] haystack, byte[] needle) {
    for (int i = 0; i + needle.length <= haystack.length; i++) {
      if (Arrays.equals(haystack, i, i + needle.length, needle, 0, needle.length)) {
        return true;
      }
    }
    return false;
  }

  @Test
  void propagates_trace_across_two_services_and_exports_spans() throws Exception {
    String traceId;
    try (FakeOtlpReceiver receiver = new FakeOtlpReceiver()) {
      try (ConfigurableApplicationContext a = start("svc-a", receiver.url());
          ConfigurableApplicationContext b = start("svc-b", receiver.url())) {
        int portB = Integer.parseInt(b.getEnvironment().getProperty("local.server.port"));
        JsonNode body = JSON.readTree(get(base(a) + "/call?port=" + portB).body());
        traceId = body.get("own").asText();
        assertThat(traceId).matches("[0-9a-f]{32}");
        assertThat(body.get("remote").asText()).isEqualTo(traceId);
      } // closing the contexts flushes the batch span processors
      byte[] id = HexFormat.of().parseHex(traceId);
      assertThat(receiver.received("/v1/traces")).anyMatch(req -> contains(req, id));
    }
  }

  @Test
  void exports_metrics_over_otlp() throws Exception {
    try (FakeOtlpReceiver receiver = new FakeOtlpReceiver();
        ConfigurableApplicationContext ctx = start("svc-m", receiver.url())) {
      SdkMeterProvider meters = ctx.getBean(SdkMeterProvider.class);
      meters.get("test").counterBuilder("requests").build().add(1);
      meters.forceFlush().join(5, TimeUnit.SECONDS);
      assertThat(receiver.received("/v1/metrics")).isNotEmpty();
    }
  }

  @Test
  void log_lines_are_json_and_carry_trace_id_inside_a_span(CapturedOutput output) throws Exception {
    try (ConfigurableApplicationContext ctx = start("svc-log", "")) {
      String traceId = get(base(ctx) + "/ping").body();

      List<String> lines = output.getOut().lines().filter(l -> !l.isBlank()).toList();
      assertThat(lines).isNotEmpty();
      for (String line : lines) {
        assertThat(JSON.readTree(line).isObject()).as(line).isTrue();
      }
      JsonNode ping =
          lines.stream()
              .map(l -> readQuietly(l))
              .filter(n -> "handling ping".equals(n.path("message").asText()))
              .findFirst()
              .orElseThrow();
      assertThat(ping.get("trace_id").asText()).isEqualTo(traceId);
    }
  }

  @ParameterizedTest
  @ValueSource(strings = {"", "not a url", "ftp://host", "http://127.0.0.1:1"})
  void starts_without_a_usable_otlp_endpoint(String endpoint) throws Exception {
    long begin = System.nanoTime();
    try (ConfigurableApplicationContext ctx = start("svc-none", endpoint)) {
      assertThat(get(base(ctx) + "/ping").body()).matches("[0-9a-f]{32}");
    }
    assertThat(Duration.ofNanos(System.nanoTime() - begin)).isLessThan(Duration.ofSeconds(20));
  }

  private static JsonNode readQuietly(String line) {
    try {
      return JSON.readTree(line);
    } catch (IOException e) {
      throw new IllegalStateException(e);
    }
  }
}
