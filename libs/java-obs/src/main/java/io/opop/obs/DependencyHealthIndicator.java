package io.opop.obs;

import java.time.Duration;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;
import java.util.concurrent.atomic.AtomicBoolean;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.actuate.health.Health;
import org.springframework.boot.actuate.health.HealthIndicator;

/**
 * Runs every {@link DependencyCheck} concurrently; DOWN if any fails or exceeds the timeout. The
 * timeout is enforced here, so a check that ignores interruption cannot delay the response, and a
 * check still running from an earlier probe is not started again (reported DOWN), which bounds
 * leaked threads to one per check. Details carry only check names and UP/DOWN; error text is
 * logged, not returned.
 */
public final class DependencyHealthIndicator implements HealthIndicator {

  /** Default bound for one readiness probe. */
  public static final Duration DEFAULT_TIMEOUT = Duration.ofSeconds(2);

  private static final Logger LOG = LoggerFactory.getLogger(DependencyHealthIndicator.class);
  private static final ExecutorService EXECUTOR = Executors.newVirtualThreadPerTaskExecutor();

  private final List<DependencyCheck> checks;
  private final Duration timeout;
  private final Map<DependencyCheck, AtomicBoolean> inFlight = new ConcurrentHashMap<>();

  public DependencyHealthIndicator(List<DependencyCheck> checks, Duration timeout) {
    this.checks = List.copyOf(checks);
    this.timeout = timeout;
  }

  @Override
  public Health health() {
    long deadline = System.nanoTime() + timeout.toNanos();
    Map<DependencyCheck, Future<?>> started = new LinkedHashMap<>();
    Map<String, String> results = new LinkedHashMap<>();
    for (DependencyCheck check : checks) {
      AtomicBoolean running = inFlight.computeIfAbsent(check, c -> new AtomicBoolean());
      if (running.compareAndSet(false, true)) {
        started.put(check, EXECUTOR.submit(() -> run(check, running)));
      } else {
        LOG.error("readiness check {} skipped: previous run still in progress", check.name());
        results.put(check.name(), "DOWN");
      }
    }
    started.forEach(
        (check, future) ->
            results.put(check.name(), await(check, future, deadline) ? "UP" : "DOWN"));
    boolean up = results.values().stream().allMatch("UP"::equals);
    return (up ? Health.up() : Health.down()).withDetails(results).build();
  }

  private static Void run(DependencyCheck check, AtomicBoolean running) throws Exception {
    try {
      check.check();
      return null;
    } finally {
      running.set(false);
    }
  }

  private boolean await(DependencyCheck check, Future<?> future, long deadline) {
    try {
      future.get(Math.max(0, deadline - System.nanoTime()), TimeUnit.NANOSECONDS);
      return true;
    } catch (TimeoutException e) {
      future.cancel(true);
      LOG.error("readiness check {} timed out after {}", check.name(), timeout);
    } catch (ExecutionException e) {
      LOG.error("readiness check {} failed", check.name(), e.getCause());
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      LOG.error("readiness check {} interrupted", check.name());
    }
    return false;
  }
}
