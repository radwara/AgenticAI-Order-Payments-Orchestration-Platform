package com.platform.payment.provider;

import static com.tngtech.archunit.lang.syntax.ArchRuleDefinition.classes;
import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.tngtech.archunit.core.domain.JavaClass;
import com.tngtech.archunit.core.importer.ClassFileImporter;
import com.tngtech.archunit.core.importer.ImportOption;
import com.tngtech.archunit.lang.ArchCondition;
import com.tngtech.archunit.lang.ArchRule;
import com.tngtech.archunit.lang.ConditionEvents;
import com.tngtech.archunit.lang.SimpleConditionEvent;
import java.io.IOException;
import java.lang.reflect.Executable;
import java.lang.reflect.Parameter;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.List;
import java.util.regex.Pattern;
import java.util.stream.Stream;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

class ArchitectureTest {

  private static final Pattern WORD_SPLIT = Pattern.compile("(?<=[a-z0-9])(?=[A-Z])|[^A-Za-z0-9]+");
  private static final Pattern BANNED_WORD =
      Pattern.compile("pan|cvv\\d?|cvc\\d?|cvn\\d?|cid|csc|track\\d?");
  private static final Pattern BANNED_NORMALISED =
      Pattern.compile(
          "cardnum|cardno|ccnum|ccno|creditcard|debitcard|primaryaccount|securitycode|"
              + "cardverification|trackdata|magstripe|magneticstripe|(expir|expdate).*(num|pan)");

  /** True if a type, field, method, parameter or constant name looks like card data. */
  static boolean isCardDataName(String name) {
    boolean bannedWord =
        Stream.of(WORD_SPLIT.split(name))
            .anyMatch(w -> BANNED_WORD.matcher(w.toLowerCase()).matches());
    String normalised = name.replaceAll("[^A-Za-z0-9]", "").toLowerCase();
    return bannedWord || BANNED_NORMALISED.matcher(normalised).find();
  }

  /**
   * Names declared by a class: itself, fields (incl. record components, enum constants), methods,
   * parameters.
   */
  private static List<String> declaredNames(JavaClass c) {
    List<String> names = new ArrayList<>(List.of(c.getSimpleName()));
    c.getFields().forEach(f -> names.add(f.getName()));
    c.getMethods().forEach(m -> names.add(m.getName()));
    Class<?> type = c.reflect();
    Stream.concat(Stream.of(type.getDeclaredMethods()), Stream.of(type.getDeclaredConstructors()))
        .map(Executable.class::cast)
        .flatMap(e -> Stream.of(e.getParameters()))
        .map(Parameter::getName)
        .forEach(names::add);
    return names;
  }

  private static final ArchRule NO_CARD_DATA =
      classes()
          .should(
              new ArchCondition<JavaClass>("not declare names that look like card data") {
                @Override
                public void check(JavaClass c, ConditionEvents events) {
                  declaredNames(c).stream()
                      .filter(ArchitectureTest::isCardDataName)
                      .forEach(
                          n ->
                              events.add(SimpleConditionEvent.violated(c, c.getName() + ": " + n)));
                }
              });

  /**
   * Fixtures used only to prove the rule can fail; test sources are never scanned in production.
   */
  @SuppressWarnings("unused")
  private static final class LeakyFields {
    String cvv2;
    String securityCode;
    String cardNum;
    String track2;
    String cardNumberValue;
    String pan_;
  }

  @SuppressWarnings("unused")
  private static final class LeakyParameter {
    void take(String primaryAccountNumber) {}
  }

  private record LeakyRecord(String cvc) {}

  private enum LeakyEnum {
    CARD_NUMBER
  }

  private static final class CardNumber {}

  @Test
  void production_code_declares_no_card_data_names() {
    var production =
        new ClassFileImporter()
            .withImportOption(ImportOption.Predefined.DO_NOT_INCLUDE_TESTS)
            .importPackages("com.platform.payment");

    assertThat(production.size()).isGreaterThan(4);
    NO_CARD_DATA.check(production);
  }

  @Test
  void the_rule_fails_for_each_evasion_in_a_leaky_fixture() {
    var leaky =
        new ClassFileImporter()
            .importClasses(
                LeakyFields.class,
                LeakyParameter.class,
                LeakyRecord.class,
                LeakyEnum.class,
                CardNumber.class);

    assertThatThrownBy(() -> NO_CARD_DATA.check(leaky))
        .isInstanceOf(AssertionError.class)
        .hasMessageContaining("cvv2")
        .hasMessageContaining("securityCode")
        .hasMessageContaining("cardNum")
        .hasMessageContaining("track2")
        .hasMessageContaining("cardNumberValue")
        .hasMessageContaining("pan_")
        .hasMessageContaining("primaryAccountNumber")
        .hasMessageContaining("cvc")
        .hasMessageContaining("CARD_NUMBER")
        .hasMessageContaining("CardNumber");
  }

  @ParameterizedTest
  @ValueSource(
      strings = {
        "pan",
        "PAN",
        "pan_",
        "_pan",
        "cvv",
        "CVV",
        "cvv2",
        "cvc",
        "cvc2",
        "cvn",
        "cid",
        "csc",
        "cardNumber",
        "card_number",
        "CARD_NUMBER",
        "cardNum",
        "cardNo",
        "cardNumberValue",
        "primaryAccountNumber",
        "securityCode",
        "SecurityCode",
        "track1",
        "track2",
        "trackData",
        "magStripe",
        "magstripeData",
        "ccNumber",
        "creditCardNumber",
        "expiryAndNumber",
        "expiryDatePan"
      })
  void the_name_matcher_rejects_each_evasion(String name) {
    assertThat(isCardDataName(name)).isTrue();
  }

  @ParameterizedTest
  @ValueSource(
      strings = {
        "expand",
        "company",
        "panel",
        "span",
        "cidr",
        "trackingId",
        "providerRef",
        "idempotencyKey",
        "INVALID_ARGUMENT",
        "CARD_DECLINED",
        "minorUnits",
        "expectedNumber"
      })
  void the_name_matcher_accepts_ordinary_names(String name) {
    assertThat(isCardDataName(name)).isFalse();
  }

  @Test
  void interface_javadoc_states_the_idempotency_expectations() throws IOException {
    String source =
        Files.readString(
            Path.of("src/main/java/com/platform/payment/provider/PaymentProvider.java"));
    String doc = source.replaceAll("(?m)^\\s*\\*\\s?", " ").replaceAll("\\s+", " ");

    assertThat(doc)
        .contains("Idempotency expectations for implementers")
        .contains("same key and the same arguments must return the same result")
        .contains("must not apply the effect twice")
        .contains("different arguments must be rejected as IDEMPOTENCY_KEY_REUSED")
        .contains("null or blank key is rejected as INVALID_ARGUMENT")
        .contains("retry must apply the effect at most once")
        .contains("never throw");
  }
}
