package io.opop.obs;

import static org.assertj.core.api.Assertions.assertThat;

import java.time.Duration;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.atomic.AtomicInteger;
import org.junit.jupiter.api.Test;
import org.springframework.boot.actuate.health.Status;

class DependencyHealthIndicatorTest {

  /** A check that ignores interruption until released, like a hung driver call. */
  private static final class HungCheck implements DependencyCheck {
    final CountDownLatch release = new CountDownLatch(1);
    final AtomicInteger starts = new AtomicInteger();

    @Override
    public String name() {
      return "hung";
    }

    @Override
    public void check() {
      starts.incrementAndGet();
      while (true) {
        try {
          release.await();
          return;
        } catch (InterruptedException ignored) {
          // keep waiting: simulates a check that cannot be cancelled
        }
      }
    }
  }

  @Test
  void times_out_a_hung_check_and_does_not_start_it_again_until_it_finishes() {
    HungCheck hung = new HungCheck();
    var indicator = new DependencyHealthIndicator(List.of(hung), Duration.ofMillis(100));

    long begin = System.nanoTime();
    assertThat(indicator.health().getStatus()).isEqualTo(Status.DOWN);
    assertThat(Duration.ofNanos(System.nanoTime() - begin)).isLessThan(Duration.ofSeconds(1));

    assertThat(indicator.health().getStatus()).isEqualTo(Status.DOWN);
    assertThat(hung.starts.get()).isEqualTo(1);

    hung.release.countDown();
    org.awaitility.Awaitility.await()
        .atMost(Duration.ofSeconds(2))
        .untilAsserted(() -> assertThat(indicator.health().getStatus()).isEqualTo(Status.UP));
  }

  @Test
  void reports_up_with_no_checks_registered() {
    var indicator = new DependencyHealthIndicator(List.of(), Duration.ofSeconds(1));
    assertThat(indicator.health().getStatus()).isEqualTo(Status.UP);
  }
}
