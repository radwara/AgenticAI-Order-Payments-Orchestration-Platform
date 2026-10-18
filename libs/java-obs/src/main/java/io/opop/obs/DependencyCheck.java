package io.opop.obs;

/**
 * A readiness check for one dependency (database, broker, ...). Register implementations as Spring
 * beans; each is run on every readiness probe.
 */
public interface DependencyCheck {

  /** Stable name reported in the readiness response. */
  String name();

  /**
   * Throws if the dependency is not ready. The thread is interrupted when the probe timeout
   * expires, so blocking calls should honour interruption. The exception message is only logged,
   * never returned to the caller.
   */
  void check() throws Exception;
}
