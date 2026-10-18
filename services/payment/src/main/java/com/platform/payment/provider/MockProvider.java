package com.platform.payment.provider;

import java.util.HashMap;
import java.util.Map;
import java.util.function.Supplier;

/**
 * Deterministic in-memory provider for sandbox and tests. The token prefix selects the outcome for
 * every operation: {@code tok_ok} succeeds, {@code tok_declined} and {@code tok_insufficient}
 * decline, {@code tok_timeout} returns a transient error. Any other token is declined as {@link
 * DeclineReason#CARD_DECLINED}.
 */
public final class MockProvider implements PaymentProvider {

  public static final String TOKEN_OK = "tok_ok";
  public static final String TOKEN_TIMEOUT = "tok_timeout";
  public static final String TOKEN_INSUFFICIENT = "tok_insufficient";

  private static final class Authorization {
    final Money authorized;
    long captured;
    long refunded;
    boolean voided;

    Authorization(Money authorized) {
      this.authorized = authorized;
    }
  }

  private final Map<String, Authorization> authorizations = new HashMap<>();
  private final Map<String, ProviderResult> results = new HashMap<>();
  private int nextRef = 1;

  @Override
  public synchronized ProviderResult authorize(String token, Money amount, String idempotencyKey) {
    return once("authorize", idempotencyKey, token, () -> doAuthorize(amount));
  }

  @Override
  public synchronized ProviderResult capture(
      String token, String providerRef, Money amount, String idempotencyKey) {
    return once("capture", idempotencyKey, token, () -> doCapture(providerRef, amount));
  }

  @Override
  public synchronized ProviderResult voidAuthorization(
      String token, String providerRef, String idempotencyKey) {
    return once("void", idempotencyKey, token, () -> doVoid(providerRef));
  }

  @Override
  public synchronized ProviderResult refund(
      String token, String providerRef, Money amount, String idempotencyKey) {
    return once("refund", idempotencyKey, token, () -> doRefund(providerRef, amount));
  }

  /** Replays the stored result for a repeated key; otherwise gates on the token, then runs. */
  private ProviderResult once(
      String operation, String key, String token, Supplier<ProviderResult> action) {
    String id = operation + ":" + key;
    ProviderResult previous = results.get(id);
    if (previous != null) {
      return previous;
    }
    ProviderResult result = gate(token);
    if (result == null) {
      result = action.get();
    }
    // Transient errors are not stored: the outcome is unknown and a retry may succeed.
    if (!(result instanceof ProviderResult.TransientError)) {
      results.put(id, result);
    }
    return result;
  }

  /** Returns the fixed result for a failing token prefix, or null when the token is good. */
  private static ProviderResult gate(String token) {
    if (token == null) {
      return declined(DeclineReason.CARD_DECLINED);
    }
    if (token.startsWith(TOKEN_OK)) {
      return null;
    }
    if (token.startsWith(TOKEN_TIMEOUT)) {
      return new ProviderResult.TransientError("mock provider timeout");
    }
    if (token.startsWith(TOKEN_INSUFFICIENT)) {
      return declined(DeclineReason.INSUFFICIENT_FUNDS);
    }
    // Covers tok_declined and any unrecognised token.
    return declined(DeclineReason.CARD_DECLINED);
  }

  private ProviderResult doAuthorize(Money amount) {
    String ref = "mock_auth_" + nextRef++;
    authorizations.put(ref, new Authorization(amount));
    return new ProviderResult.Success(ref, amount);
  }

  private ProviderResult doCapture(String ref, Money amount) {
    Authorization auth = authorizations.get(ref);
    ProviderResult invalid = validate(auth, amount);
    if (invalid != null) {
      return invalid;
    }
    if (auth.voided) {
      return declined(DeclineReason.INVALID_STATE);
    }
    if (auth.captured + amount.minorUnits() > auth.authorized.minorUnits()) {
      return declined(DeclineReason.AMOUNT_EXCEEDS_AUTH);
    }
    auth.captured += amount.minorUnits();
    return new ProviderResult.Success(ref, amount);
  }

  private ProviderResult doVoid(String ref) {
    Authorization auth = authorizations.get(ref);
    if (auth == null) {
      return declined(DeclineReason.UNKNOWN_REFERENCE);
    }
    if (auth.voided || auth.captured > 0) {
      return declined(DeclineReason.INVALID_STATE);
    }
    auth.voided = true;
    return new ProviderResult.Success(ref, auth.authorized);
  }

  private ProviderResult doRefund(String ref, Money amount) {
    Authorization auth = authorizations.get(ref);
    ProviderResult invalid = validate(auth, amount);
    if (invalid != null) {
      return invalid;
    }
    if (auth.refunded + amount.minorUnits() > auth.captured) {
      return declined(DeclineReason.AMOUNT_EXCEEDS_CAPTURE);
    }
    auth.refunded += amount.minorUnits();
    return new ProviderResult.Success(ref, amount);
  }

  private static ProviderResult validate(Authorization auth, Money amount) {
    if (auth == null) {
      return declined(DeclineReason.UNKNOWN_REFERENCE);
    }
    if (!auth.authorized.currency().equals(amount.currency())) {
      return declined(DeclineReason.CURRENCY_MISMATCH);
    }
    return null;
  }

  private static ProviderResult declined(DeclineReason reason) {
    return new ProviderResult.Declined(reason);
  }
}
