package com.platform.payment.provider.sandbox;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.platform.payment.provider.DeclineReason;
import com.platform.payment.provider.Money;
import com.platform.payment.provider.ProviderResult;
import com.platform.payment.provider.ProviderResult.Declined;
import com.platform.payment.provider.ProviderResult.Success;
import com.platform.payment.provider.ProviderResult.TransientError;
import java.lang.reflect.Modifier;
import java.util.stream.Stream;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;
import org.junit.jupiter.params.provider.ValueSource;

class MockProviderTest {

  private static final Money TEN = new Money(1000, "USD");
  private static final String OK = "tok_ok";
  private MockProvider p;
  private int keys;

  @BeforeEach
  void setUp() {
    p = MockProvider.sandboxOnly();
  }

  private String key() {
    return "key-" + keys++;
  }

  private static Money usd(long minor) {
    return new Money(minor, "USD");
  }

  private static Declined declined(DeclineReason reason) {
    return new Declined(reason);
  }

  private String auth(String token, Money amount) {
    return ((Success) p.authorize(token, amount, key())).providerRef();
  }

  private String auth(Money amount) {
    return auth(OK, amount);
  }

  private String captured(Money authorized, Money captured) {
    String ref = auth(authorized);
    assertThat(p.capture(OK, ref, captured, key())).isInstanceOf(Success.class);
    return ref;
  }

  private enum Op {
    AUTHORIZE,
    CAPTURE,
    VOID,
    REFUND
  }

  /** Runs one operation with the token, on a reference prepared with a good token of that kind. */
  private ProviderResult run(Op op, String token) {
    String good = token.startsWith(OK) ? token : OK;
    String ref = auth(good, TEN);
    return switch (op) {
      case AUTHORIZE -> p.authorize(token, TEN, key());
      case CAPTURE -> p.capture(token, ref, TEN, key());
      case VOID -> p.voidAuthorization(token, ref, key());
      case REFUND -> {
        p.capture(good, ref, TEN, key());
        yield p.refund(token, ref, TEN, key());
      }
    };
  }

  static Stream<Arguments> prefixesAndOperations() {
    return Stream.of(Op.values())
        .flatMap(
            op ->
                Stream.of(
                    Arguments.of(op, "tok_ok", Success.class),
                    Arguments.of(op, "tok_ok_visa", Success.class),
                    Arguments.of(op, "tok_declined", Declined.class),
                    Arguments.of(op, "tok_insufficient", Declined.class),
                    Arguments.of(op, "tok_timeout", TransientError.class)));
  }

  @ParameterizedTest(name = "{0} with {1}")
  @MethodSource("prefixesAndOperations")
  void each_token_prefix_yields_its_documented_result_for_every_operation(
      Op op, String token, Class<?> expected) {
    assertThat(run(op, token)).isInstanceOf(expected);
  }

  @Test
  void declined_tokens_carry_their_documented_reason() {
    assertThat(p.authorize("tok_declined", TEN, key()))
        .isEqualTo(declined(DeclineReason.CARD_DECLINED));
    assertThat(p.authorize("tok_insufficient", TEN, key()))
        .isEqualTo(declined(DeclineReason.INSUFFICIENT_FUNDS));
    assertThat(p.authorize("whatever", TEN, key()))
        .isEqualTo(declined(DeclineReason.CARD_DECLINED));
  }

  @Test
  void rejects_capture_above_authorized_amount_including_overflow() {
    String ref = auth(usd(100));
    assertThat(p.capture(OK, ref, usd(50), key())).isInstanceOf(Success.class);

    assertThat(p.capture(OK, ref, usd(51), key()))
        .isEqualTo(declined(DeclineReason.AMOUNT_EXCEEDS_AUTH));
    assertThat(p.capture(OK, ref, usd(Long.MAX_VALUE), key()))
        .isEqualTo(declined(DeclineReason.AMOUNT_EXCEEDS_AUTH));
    assertThat(p.capture(OK, ref, usd(50), key())).isInstanceOf(Success.class);
    assertThat(p.capture(OK, ref, usd(1), key()))
        .isEqualTo(declined(DeclineReason.AMOUNT_EXCEEDS_AUTH));
  }

  @Test
  void rejects_refund_total_above_captured_amount_including_overflow() {
    String ref = captured(usd(Long.MAX_VALUE), usd(700));
    assertThat(p.refund(OK, ref, usd(500), key())).isInstanceOf(Success.class);

    assertThat(p.refund(OK, ref, usd(201), key()))
        .isEqualTo(declined(DeclineReason.AMOUNT_EXCEEDS_CAPTURE));
    assertThat(p.refund(OK, ref, usd(Long.MAX_VALUE), key()))
        .isEqualTo(declined(DeclineReason.AMOUNT_EXCEEDS_CAPTURE));
    assertThat(p.refund(OK, ref, usd(200), key())).isInstanceOf(Success.class);
    assertThat(p.refund(OK, auth(TEN), usd(1), key()))
        .isEqualTo(declined(DeclineReason.AMOUNT_EXCEEDS_CAPTURE));
  }

  @Test
  void replaying_a_key_returns_the_first_result_without_applying_the_effect_twice() {
    String ref = auth(TEN);
    ProviderResult first = p.capture(OK, ref, usd(600), "same");

    assertThat(p.capture(OK, ref, usd(600), "same")).isEqualTo(first);
    assertThat(p.capture(OK, ref, usd(400), key())).isInstanceOf(Success.class);
  }

  @Test
  void reusing_a_key_with_different_arguments_is_rejected() {
    String ref = auth(TEN);
    String other = auth(TEN);
    p.capture(OK, ref, usd(600), "same");
    DeclineReason reused = DeclineReason.IDEMPOTENCY_KEY_REUSED;

    assertThat(p.capture(OK, ref, usd(100), "same")).isEqualTo(declined(reused));
    assertThat(p.capture(OK, other, usd(600), "same")).isEqualTo(declined(reused));
    assertThat(p.capture("tok_ok_2", ref, usd(600), "same")).isEqualTo(declined(reused));
    assertThat(p.refund(OK, ref, usd(600), "same")).isEqualTo(declined(reused));
    // The rejected reuses applied nothing: the full 600 is still refundable, and no more.
    assertThat(p.refund(OK, ref, usd(600), key())).isInstanceOf(Success.class);
    assertThat(p.refund(OK, ref, usd(1), key()))
        .isEqualTo(declined(DeclineReason.AMOUNT_EXCEEDS_CAPTURE));
  }

  @ParameterizedTest
  @ValueSource(strings = {"", "  "})
  void rejects_blank_and_null_idempotency_keys(String blank) {
    assertThat(p.authorize(OK, TEN, blank)).isEqualTo(declined(DeclineReason.INVALID_ARGUMENT));
    assertThat(p.authorize(OK, TEN, null)).isEqualTo(declined(DeclineReason.INVALID_ARGUMENT));
    assertThat(p.voidAuthorization(OK, "x", null))
        .isEqualTo(declined(DeclineReason.INVALID_ARGUMENT));
  }

  @Test
  void returns_invalid_argument_instead_of_throwing_for_bad_inputs() {
    String ref = captured(TEN, usd(500));
    Declined invalid = declined(DeclineReason.INVALID_ARGUMENT);

    assertThat(p.authorize(OK, null, key())).isEqualTo(invalid);
    assertThat(p.authorize(OK, usd(0), key())).isEqualTo(invalid);
    assertThat(p.capture(OK, ref, null, key())).isEqualTo(invalid);
    assertThat(p.capture(OK, ref, usd(0), key())).isEqualTo(invalid);
    assertThat(p.refund(OK, ref, usd(0), key())).isEqualTo(invalid);
    assertThat(p.refund(OK, ref, null, key())).isEqualTo(invalid);
    assertThat(p.authorize(null, TEN, key())).isEqualTo(invalid);
    assertThat(p.authorize(" ", TEN, key())).isEqualTo(invalid);
    assertThat(p.capture(OK, null, TEN, key()))
        .isEqualTo(declined(DeclineReason.UNKNOWN_REFERENCE));
  }

  @Test
  void rejects_unknown_reference_wrong_token_currency_mismatch_and_bad_state() {
    String ref = auth(TEN);

    assertThat(p.capture(OK, "nope", TEN, key()))
        .isEqualTo(declined(DeclineReason.UNKNOWN_REFERENCE));
    assertThat(p.voidAuthorization(OK, "nope", key()))
        .isEqualTo(declined(DeclineReason.UNKNOWN_REFERENCE));
    assertThat(p.capture("tok_ok_other", ref, TEN, key()))
        .isEqualTo(declined(DeclineReason.UNKNOWN_REFERENCE));
    assertThat(p.capture(OK, ref, new Money(1, "EUR"), key()))
        .isEqualTo(declined(DeclineReason.CURRENCY_MISMATCH));
    assertThat(p.voidAuthorization(OK, ref, key())).isInstanceOf(Success.class);
    assertThat(p.voidAuthorization(OK, ref, key()))
        .isEqualTo(declined(DeclineReason.INVALID_STATE));
    assertThat(p.capture(OK, ref, TEN, key())).isEqualTo(declined(DeclineReason.INVALID_STATE));
    assertThat(p.voidAuthorization(OK, captured(TEN, TEN), key()))
        .isEqualTo(declined(DeclineReason.INVALID_STATE));
  }

  @Test
  void transient_errors_are_not_remembered() {
    assertThat(p.authorize("tok_timeout", TEN, "k")).isInstanceOf(TransientError.class);
    assertThat(p.authorize("tok_timeout", TEN, "k")).isInstanceOf(TransientError.class);
  }

  @Test
  void can_only_be_created_through_the_sandbox_factory() {
    assertThat(MockProvider.class.getDeclaredConstructors())
        .allMatch(c -> Modifier.isPrivate(c.getModifiers()));
  }

  @ParameterizedTest
  @ValueSource(strings = {"XXX", "XTS", "XAU", "XXY", "usd", ""})
  void money_rejects_pseudo_and_unknown_currencies(String code) {
    assertThatThrownBy(() -> new Money(1, code)).isInstanceOf(IllegalArgumentException.class);
  }

  @Test
  void money_rejects_negative_amounts_and_null_currency() {
    assertThatThrownBy(() -> new Money(-1, "USD")).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> new Money(1, null)).isInstanceOf(IllegalArgumentException.class);
  }
}
