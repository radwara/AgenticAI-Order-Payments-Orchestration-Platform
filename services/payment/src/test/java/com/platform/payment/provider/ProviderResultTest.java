package com.platform.payment.provider;

import static org.assertj.core.api.Assertions.assertThat;

import com.platform.payment.provider.ProviderResult.Declined;
import com.platform.payment.provider.ProviderResult.Success;
import com.platform.payment.provider.ProviderResult.TransientError;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;
import org.junit.jupiter.api.Test;

class ProviderResultTest {

  /** No default branch: adding a permitted subtype makes this fail to compile. */
  private static String describe(ProviderResult result) {
    return switch (result) {
      case Success s -> "success " + s.providerRef();
      case Declined d -> "declined " + d.reason();
      case TransientError t -> "transient " + t.message();
    };
  }

  @Test
  void switch_over_the_sealed_hierarchy_is_exhaustive() {
    List<ProviderResult> results =
        List.of(
            new Success("ref", new Money(1, "USD")),
            new Declined(DeclineReason.CARD_DECLINED),
            new TransientError("timeout"));

    assertThat(results)
        .extracting(ProviderResultTest::describe)
        .containsExactly("success ref", "declined CARD_DECLINED", "transient timeout");
    assertThat(ProviderResult.class.getPermittedSubclasses()).hasSize(3);
  }

  @Test
  void every_decline_reason_is_documented() throws IOException {
    String source =
        Files.readString(Path.of("src/main/java/com/platform/payment/provider/DeclineReason.java"));

    for (DeclineReason reason : DeclineReason.values()) {
      assertThat(source).containsPattern("\\*/\\s*" + reason.name() + "[,\\s]");
    }
  }
}
