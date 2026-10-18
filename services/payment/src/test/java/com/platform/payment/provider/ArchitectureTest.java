package com.platform.payment.provider;

import static com.tngtech.archunit.lang.syntax.ArchRuleDefinition.fields;
import static org.assertj.core.api.Assertions.assertThat;

import com.tngtech.archunit.core.importer.ImportOption;
import com.tngtech.archunit.junit.AnalyzeClasses;
import com.tngtech.archunit.junit.ArchTest;
import com.tngtech.archunit.lang.ArchRule;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import org.junit.jupiter.api.Test;

@AnalyzeClasses(
    packages = "com.platform.payment",
    importOptions = ImportOption.DoNotIncludeTests.class)
class ArchitectureTest {

  @ArchTest
  static final ArchRule no_card_data_fields =
      fields()
          .should()
          .haveNameNotMatching("(?i)(pan|cvv|cvc|cardNumber|card_number)")
          .because("raw card data must never exist in any type");

  @Test
  void interface_javadoc_lists_idempotency_expectations() throws IOException {
    Path source = Path.of("src/main/java/com/platform/payment/provider/PaymentProvider.java");

    assertThat(Files.readString(source))
        .contains("Idempotency expectations for implementers")
        .contains("idempotencyKey");
  }
}
