package com.platform.payment.provider;

import java.util.Currency;

/**
 * Monetary amount as integer minor units (for example cents) plus an ISO-4217 currency code. Never
 * uses floating point. Zero is representable; operations that require a positive amount reject it.
 *
 * <p>Null contract: a null currency is rejected with {@link IllegalArgumentException}; the helpers
 * reject a null argument with {@link NullPointerException}. Providers must validate before
 * constructing and return a Declined result rather than let these escape.
 *
 * @param minorUnits amount in minor units, zero or greater
 * @param currency ISO-4217 code of a real currency; pseudo-currencies such as XXX, XTS and XAU are
 *     rejected
 */
public record Money(long minorUnits, String currency) {

  public Money {
    if (minorUnits < 0) {
      throw new IllegalArgumentException("minorUnits must not be negative");
    }
    if (currency == null) {
      throw new IllegalArgumentException("currency is required");
    }
    // Throws IllegalArgumentException for unknown codes; pseudo-currencies have no minor units.
    if (Currency.getInstance(currency).getDefaultFractionDigits() < 0) {
      throw new IllegalArgumentException("not a real currency: " + currency);
    }
  }

  /**
   * Returns this plus {@code other}.
   *
   * @throws ArithmeticException if the sum overflows {@code long}
   * @throws IllegalArgumentException if the currencies differ
   */
  public Money plus(Money other) {
    requireSameCurrency(other);
    return new Money(Math.addExact(minorUnits, other.minorUnits), currency);
  }

  /**
   * Returns this minus {@code other}.
   *
   * @throws IllegalArgumentException if the currencies differ or the result would be negative
   */
  public Money minus(Money other) {
    requireSameCurrency(other);
    return new Money(Math.subtractExact(minorUnits, other.minorUnits), currency);
  }

  private void requireSameCurrency(Money other) {
    if (!currency.equals(other.currency)) {
      throw new IllegalArgumentException(
          "currency mismatch: " + currency + " vs " + other.currency);
    }
  }
}
