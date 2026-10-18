package io.opop.obs;

import ch.qos.logback.classic.spi.ILoggingEvent;
import ch.qos.logback.classic.spi.ThrowableProxyUtil;
import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.Map;
import org.springframework.boot.json.JsonWriter;
import org.springframework.boot.logging.structured.StructuredLogFormatter;

/**
 * One JSON object per line. When the logging thread is inside a span, {@code trace_id} and {@code
 * span_id} are taken from the MDC keys that the Micrometer tracing bridge populates.
 */
public final class TraceJsonLogFormatter implements StructuredLogFormatter<ILoggingEvent> {

  private static final String MDC_TRACE_ID = "traceId";
  private static final String MDC_SPAN_ID = "spanId";
  private static final JsonWriter<Object> WRITER = JsonWriter.standard();

  @Override
  public String format(ILoggingEvent event) {
    Map<String, Object> line = new LinkedHashMap<>();
    line.put("timestamp", Instant.ofEpochMilli(event.getTimeStamp()).toString());
    line.put("level", event.getLevel().toString());
    line.put("logger", event.getLoggerName());
    line.put("message", event.getFormattedMessage());
    Map<String, String> mdc = event.getMDCPropertyMap();
    putIfPresent(line, "trace_id", mdc.get(MDC_TRACE_ID));
    putIfPresent(line, "span_id", mdc.get(MDC_SPAN_ID));
    if (event.getThrowableProxy() != null) {
      line.put("stack_trace", ThrowableProxyUtil.asString(event.getThrowableProxy()));
    }
    return WRITER.writeToString(line) + "\n";
  }

  private static void putIfPresent(Map<String, Object> line, String key, String value) {
    if (value != null && !value.isEmpty()) {
      line.put(key, value);
    }
  }
}
