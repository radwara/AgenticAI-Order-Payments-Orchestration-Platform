package com.platform.payment.provider;

/** Closed set of outcomes of a provider operation. */
public sealed interface ProviderResult {

  /**
   * The provider accepted the operation.
   *
   * @param providerRef provider-side reference of the authorization
   * @param amount amount the operation applied to
   */
  record Success(String providerRef, Money amount) implements ProviderResult {}

  /**
   * The provider definitively refused the operation; do not retry.
   *
   * @param reason why it was refused
   */
  record Declined(DeclineReason reason) implements ProviderResult {}

  /**
   * Temporary failure (timeout, provider unavailable). The outcome is unknown, so the caller may
   * retry with the same idempotency key.
   *
   * @param message human-readable detail, never card data
   */
  record TransientError(String message) implements ProviderResult {}
}
