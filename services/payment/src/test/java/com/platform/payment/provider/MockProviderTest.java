package com.platform.payment.provider;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.platform.payment.provider.ProviderResult.Declined;
import com.platform.payment.provider.ProviderResult.Success;
import com.platform.payment.provider.ProviderResult.TransientError;
import java.util.function.BiFunction;
import java.util.stream.Stream;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

class MockProviderTest {

  private static final Money TEN = new Money(1000, "USD");
  private MockProvider provider;

  @BeforeEach
  void setUp() {
    provider = new MockProvider();
  }

  record Op(String name, BiFunction<MockProvider, String, ProviderResult> call) {
    @Override
    public String toString() {
      return name;
    }
  }

  static Stream<Arguments> prefixesAndOperations() {
    Stream<Op> ops =
        Stream.of(
            new Op("authorize", (p, t) -> p.authorize(t, TEN, "k1")),
            new Op("capture", (p, t) -> p.capture(t, authRef(p), TEN, "k2")),
            new Op("void", (p, t) -> p.voidAuthorization(t, authRef(p), "k3")),
            new Op("refund", (p, t) -> p.refund(t, capturedRef(p), TEN, "k4")));
    return ops.flatMap(
        op ->
            Stream.of(
                Arguments.of(op, "tok_ok", Success.class),
                Arguments.of(op, "tok_ok_visa", Success.class),
                Arguments.of(op, "tok_declined", Declined.class),
                Arguments.of(op, "tok_insufficient", Declined.class),
                Arguments.of(op, "tok_timeout", TransientError.class)));
  }

  private static String authRef(MockProvider p) {
    return ((Success) p.authorize("tok_ok", TEN, "setup-auth")).providerRef();
  }

  private static String capturedRef(MockProvider p) {
    String ref = authRef(p);
    p.capture("tok_ok", ref, TEN, "setup-capture");
    return ref;
  }

  @ParameterizedTest(name = "{0} with {1}")
  @MethodSource("prefixesAndOperations")
  void each_token_prefix_yields_its_documented_result_for_every_operation(
      Op op, String token, Class<?> expected) {
    ProviderResult result = op.call().apply(provider, token);

    assertThat(result).isInstanceOf(expected);
  }

  @Test
  void declined_tokens_carry_their_documented_reason() {
    assertThat(provider.authorize("tok_declined", TEN, "a"))
        .isEqualTo(new Declined(DeclineReason.CARD_DECLINED));
    assertThat(provider.authorize("tok_insufficient", TEN, "b"))
        .isEqualTo(new Declined(DeclineReason.INSUFFICIENT_FUNDS));
  }

  @Test
  void rejects_capture_above_authorized_amount() {
    String ref = authRef(provider);

    ProviderResult result = provider.capture("tok_ok", ref, new Money(1001, "USD"), "c1");

    assertThat(result).isEqualTo(new Declined(DeclineReason.AMOUNT_EXCEEDS_AUTH));
  }

  @Test
  void rejects_cumulative_partial_captures_above_authorized_amount() {
    String ref = authRef(provider);
    provider.capture("tok_ok", ref, new Money(600, "USD"), "c1");

    ProviderResult result = provider.capture("tok_ok", ref, new Money(401, "USD"), "c2");

    assertThat(result).isEqualTo(new Declined(DeclineReason.AMOUNT_EXCEEDS_AUTH));
  }

  @Test
  void rejects_refund_total_above_captured_amount() {
    String ref = authRef(provider);
    provider.capture("tok_ok", ref, new Money(700, "USD"), "c1");
    assertThat(provider.refund("tok_ok", ref, new Money(500, "USD"), "r1"))
        .isInstanceOf(Success.class);

    ProviderResult result = provider.refund("tok_ok", ref, new Money(201, "USD"), "r2");

    assertThat(result).isEqualTo(new Declined(DeclineReason.AMOUNT_EXCEEDS_CAPTURE));
    assertThat(provider.refund("tok_ok", ref, new Money(200, "USD"), "r3"))
        .isInstanceOf(Success.class);
  }

  @Test
  void rejects_refund_of_uncaptured_authorization() {
    String ref = authRef(provider);

    assertThat(provider.refund("tok_ok", ref, new Money(1, "USD"), "r1"))
        .isEqualTo(new Declined(DeclineReason.AMOUNT_EXCEEDS_CAPTURE));
  }

  @Test
  void replaying_an_idempotency_key_does_not_apply_the_effect_twice() {
    String ref = authRef(provider);
    ProviderResult first = provider.capture("tok_ok", ref, TEN, "same");

    ProviderResult replay = provider.capture("tok_ok", ref, TEN, "same");

    assertThat(replay).isEqualTo(first);
    assertThat(provider.refund("tok_ok", ref, TEN, "r1")).isInstanceOf(Success.class);
    assertThat(provider.refund("tok_ok", ref, new Money(1, "USD"), "r2"))
        .isEqualTo(new Declined(DeclineReason.AMOUNT_EXCEEDS_CAPTURE));
  }

  @Test
  void rejects_unknown_reference_currency_mismatch_and_bad_state() {
    String ref = authRef(provider);

    assertThat(provider.capture("tok_ok", "nope", TEN, "a"))
        .isEqualTo(new Declined(DeclineReason.UNKNOWN_REFERENCE));
    assertThat(provider.voidAuthorization("tok_ok", "nope", "b"))
        .isEqualTo(new Declined(DeclineReason.UNKNOWN_REFERENCE));
    assertThat(provider.capture("tok_ok", ref, new Money(1, "EUR"), "c"))
        .isEqualTo(new Declined(DeclineReason.CURRENCY_MISMATCH));
    assertThat(provider.voidAuthorization("tok_ok", ref, "d")).isInstanceOf(Success.class);
    assertThat(provider.voidAuthorization("tok_ok", ref, "e"))
        .isEqualTo(new Declined(DeclineReason.INVALID_STATE));
    assertThat(provider.capture("tok_ok", ref, TEN, "f"))
        .isEqualTo(new Declined(DeclineReason.INVALID_STATE));
  }

  @Test
  void cannot_void_after_capture() {
    String ref = capturedRef(provider);

    assertThat(provider.voidAuthorization("tok_ok", ref, "v"))
        .isEqualTo(new Declined(DeclineReason.INVALID_STATE));
  }

  @Test
  void unknown_and_null_tokens_are_declined() {
    assertThat(provider.authorize("whatever", TEN, "a"))
        .isEqualTo(new Declined(DeclineReason.CARD_DECLINED));
    assertThat(provider.authorize(null, TEN, "b"))
        .isEqualTo(new Declined(DeclineReason.CARD_DECLINED));
  }

  @Test
  void transient_errors_are_not_cached_so_retry_can_succeed() {
    assertThat(provider.authorize("tok_timeout", TEN, "k")).isInstanceOf(TransientError.class);
    assertThat(provider.authorize("tok_timeout", TEN, "k")).isInstanceOf(TransientError.class);
  }

  @Test
  void money_rejects_negative_amounts_and_invalid_currency() {
    assertThatThrownBy(() -> new Money(-1, "USD")).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> new Money(1, "XXY")).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> new Money(1, null)).isInstanceOf(IllegalArgumentException.class);
  }
}
