package io.opop.obs;

import com.sun.net.httpserver.HttpServer;
import java.io.IOException;
import java.net.InetSocketAddress;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;

/** In-process OTLP/HTTP receiver that records raw request bodies per path. */
final class FakeOtlpReceiver implements AutoCloseable {

  private final HttpServer server;
  private final Map<String, List<byte[]>> bodies = new ConcurrentHashMap<>();

  FakeOtlpReceiver() throws IOException {
    server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
    server.createContext(
        "/",
        exchange -> {
          bodies
              .computeIfAbsent(
                  exchange.getRequestURI().getPath(), p -> new CopyOnWriteArrayList<>())
              .add(exchange.getRequestBody().readAllBytes());
          exchange.sendResponseHeaders(200, -1);
          exchange.close();
        });
    server.start();
  }

  String url() {
    return "http://127.0.0.1:" + server.getAddress().getPort();
  }

  /** Raw bodies received on the given path, e.g. {@code /v1/traces}. */
  List<byte[]> received(String path) {
    return bodies.getOrDefault(path, List.of());
  }

  @Override
  public void close() {
    server.stop(0);
  }
}
