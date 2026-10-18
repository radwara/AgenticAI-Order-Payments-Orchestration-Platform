package com.platform.payment.provider;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

class MoneyTest {

  private static Money usd(long minor) {
    return new Money(minor, "USD");
  }

  @ParameterizedTest
  @ValueSource(strings = {"XXX", "XTS", "XAU", "XXY", "usd", ""})
  void rejects_pseudo_and_unknown_currencies(String code) {
    assertThatThrownBy(() -> new Money(1, code)).isInstanceOf(IllegalArgumentException.class);
  }

  @Test
  void rejects_null_currency_and_negative_amounts() {
    assertThatThrownBy(() -> new Money(1, null)).isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> new Money(Long.MIN_VALUE, "USD"))
        .isInstanceOf(IllegalArgumentException.class);
  }

  @Test
  void adds_and_subtracts_exactly() {
    assertThat(usd(5).plus(usd(7))).isEqualTo(usd(12));
    assertThat(usd(Long.MAX_VALUE).minus(usd(Long.MAX_VALUE))).isEqualTo(usd(0));
  }

  @Test
  void rejects_overflow_and_negative_results() {
    assertThatThrownBy(() -> usd(Long.MAX_VALUE).plus(usd(1)))
        .isInstanceOf(ArithmeticException.class);
    assertThatThrownBy(() -> usd(1).minus(usd(2))).isInstanceOf(IllegalArgumentException.class);
  }

  @Test
  void rejects_mixed_currencies_and_null_operands() {
    assertThatThrownBy(() -> usd(1).plus(new Money(1, "EUR")))
        .isInstanceOf(IllegalArgumentException.class);
    assertThatThrownBy(() -> usd(1).minus(null)).isInstanceOf(NullPointerException.class);
  }
}
