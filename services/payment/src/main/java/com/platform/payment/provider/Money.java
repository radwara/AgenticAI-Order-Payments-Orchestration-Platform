package com.platform.payment.provider;

import java.util.Currency;

/**
 * Monetary amount as integer minor units (for example cents) plus an ISO-4217 currency code. Never
 * uses floating point. Zero is representable; operations that require a positive amount reject it.
 *
 * @param minorUnits amount in minor units, zero or greater
 * @param currency ISO-4217 code of a real currency; pseudo-currencies such as XXX and XTS are
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
}
