package com.platform.payment.provider;

import java.util.Currency;

/**
 * Monetary amount as integer minor units (for example cents) plus an ISO-4217 currency code. Never
 * uses floating point.
 *
 * @param minorUnits amount in minor units, zero or greater
 * @param currency ISO-4217 alphabetic code, for example {@code USD}
 */
public record Money(long minorUnits, String currency) {

  public Money {
    if (minorUnits < 0) {
      throw new IllegalArgumentException("minorUnits must not be negative");
    }
    if (currency == null) {
      throw new IllegalArgumentException("currency is required");
    }
    // Throws IllegalArgumentException for codes that are not ISO-4217.
    Currency.getInstance(currency);
  }
}
