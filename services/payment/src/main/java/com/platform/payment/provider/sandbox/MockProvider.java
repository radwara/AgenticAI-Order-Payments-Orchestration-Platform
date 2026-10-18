package com.platform.payment.provider.sandbox;

import com.platform.payment.provider.DeclineReason;
import com.platform.payment.provider.Money;
import com.platform.payment.provider.PaymentProvider;
import com.platform.payment.provider.ProviderResult;
import java.util.HashMap;
import java.util.Map;
import java.util.function.Supplier;

/**
 * Deterministic in-memory provider for sandbox and tests only; never wire it into production. Use
 * {@link #sandboxOnly()}.
 *
 * <p>The token prefix (plain {@code startsWith}, checked in this order) selects the outcome of
 * every operation: {@code tok_ok} succeeds, {@code tok_timeout} returns a transient error, {@code
 * tok_insufficient} declines with INSUFFICIENT_FUNDS, and any other token (including {@code
 * tok_declined}) declines with CARD_DECLINED. A null or blank token is INVALID_ARGUMENT. A
 * successful authorization is bound to its token; other tokens get UNKNOWN_REFERENCE for it.
 *
 * <p>Amounts must be positive (zero is INVALID_ARGUMENT). A transient error applies no effect and
 * is not remembered, so a retry with the same key is evaluated afresh.
 */
public final class MockProvider implements PaymentProvider {

  public static final String TOKEN_OK = "tok_ok";
  public static final String TOKEN_TIMEOUT = "tok_timeout";
  public static final String TOKEN_INSUFFICIENT = "tok_insufficient";

  private record Request(String operation, String token, String providerRef, Money amount) {}

  private record Entry(Request request, ProviderResult result) {}

  private static final class Authorization {
    final String token;
    final Money authorized;
    long captured;
    long refunded;
    boolean voided;

    Authorization(String token, Money authorized) {
      this.token = token;
      this.authorized = authorized;
    }
  }

  private final Map<String, Authorization> authorizations = new HashMap<>();
  private final Map<String, Entry> entries = new HashMap<>();
  private int nextRef = 1;

  private MockProvider() {}

  /** Creates a mock provider; the name is the acknowledgement that it is for sandbox use. */
  public static MockProvider sandboxOnly() {
    return new MockProvider();
  }

  @Override
  public synchronized ProviderResult authorize(String token, Money amount, String key) {
    return once(
        key, new Request("authorize", token, null, amount), () -> doAuthorize(token, amount));
  }

  @Override
  public synchronized ProviderResult capture(String token, String ref, Money amount, String key) {
    return once(
        key, new Request("capture", token, ref, amount), () -> doCapture(token, ref, amount));
  }

  @Override
  public synchronized ProviderResult voidAuthorization(String token, String ref, String key) {
    return once(key, new Request("void", token, ref, null), () -> doVoid(token, ref));
  }

  @Override
  public synchronized ProviderResult refund(String token, String ref, Money amount, String key) {
    return once(key, new Request("refund", token, ref, amount), () -> doRefund(token, ref, amount));
  }

  /** Replays a stored result for the same key and request; a different request is rejected. */
  private ProviderResult once(String key, Request request, Supplier<ProviderResult> action) {
    if (key == null || key.isBlank()) {
      return declined(DeclineReason.INVALID_ARGUMENT);
    }
    Entry previous = entries.get(key);
    if (previous != null) {
      return previous.request().equals(request)
          ? previous.result()
          : declined(DeclineReason.IDEMPOTENCY_KEY_REUSED);
    }
    ProviderResult result = gate(request.token());
    if (result == null) {
      result = action.get();
    }
    if (!(result instanceof ProviderResult.TransientError)) {
      entries.put(key, new Entry(request, result));
    }
    return result;
  }

  /** Returns the fixed result for a failing token, or null when the token is good. */
  private static ProviderResult gate(String token) {
    if (token == null || token.isBlank()) {
      return declined(DeclineReason.INVALID_ARGUMENT);
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
    return declined(DeclineReason.CARD_DECLINED);
  }

  private ProviderResult doAuthorize(String token, Money amount) {
    if (!positive(amount)) {
      return declined(DeclineReason.INVALID_ARGUMENT);
    }
    String ref = "mock_auth_" + nextRef++;
    authorizations.put(ref, new Authorization(token, amount));
    return new ProviderResult.Success(ref, amount);
  }

  private ProviderResult doCapture(String token, String ref, Money amount) {
    Authorization auth = find(token, ref);
    ProviderResult invalid = validate(auth, amount);
    if (invalid != null) {
      return invalid;
    }
    if (auth.voided) {
      return declined(DeclineReason.INVALID_STATE);
    }
    // Compare before adding so the arithmetic cannot overflow (captured <= authorized).
    if (amount.minorUnits() > auth.authorized.minorUnits() - auth.captured) {
      return declined(DeclineReason.AMOUNT_EXCEEDS_AUTH);
    }
    auth.captured += amount.minorUnits();
    return new ProviderResult.Success(ref, amount);
  }

  private ProviderResult doVoid(String token, String ref) {
    Authorization auth = find(token, ref);
    if (auth == null) {
      return declined(DeclineReason.UNKNOWN_REFERENCE);
    }
    if (auth.voided || auth.captured > 0) {
      return declined(DeclineReason.INVALID_STATE);
    }
    auth.voided = true;
    return new ProviderResult.Success(ref, auth.authorized);
  }

  private ProviderResult doRefund(String token, String ref, Money amount) {
    Authorization auth = find(token, ref);
    ProviderResult invalid = validate(auth, amount);
    if (invalid != null) {
      return invalid;
    }
    if (amount.minorUnits() > auth.captured - auth.refunded) {
      return declined(DeclineReason.AMOUNT_EXCEEDS_CAPTURE);
    }
    auth.refunded += amount.minorUnits();
    return new ProviderResult.Success(ref, amount);
  }

  private Authorization find(String token, String ref) {
    Authorization auth = authorizations.get(ref);
    return auth != null && auth.token.equals(token) ? auth : null;
  }

  private static ProviderResult validate(Authorization auth, Money amount) {
    if (!positive(amount)) {
      return declined(DeclineReason.INVALID_ARGUMENT);
    }
    if (auth == null) {
      return declined(DeclineReason.UNKNOWN_REFERENCE);
    }
    if (!auth.authorized.currency().equals(amount.currency())) {
      return declined(DeclineReason.CURRENCY_MISMATCH);
    }
    return null;
  }

  private static boolean positive(Money amount) {
    return amount != null && amount.minorUnits() > 0;
  }

  private static ProviderResult declined(DeclineReason reason) {
    return new ProviderResult.Declined(reason);
  }
}
