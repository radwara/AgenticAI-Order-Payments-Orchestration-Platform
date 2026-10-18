package com.agenticpay.order.domain;

import java.util.EnumMap;
import java.util.EnumSet;
import java.util.Map;
import java.util.Set;

/**
 * The only authority on legal order status changes. Anything not in the table is an error, never a
 * no-op.
 *
 * <pre>
 * from        | allowed targets
 * ------------+------------------------------------
 * CREATED     | PRICED, CANCELLED, FAILED
 * PRICED      | RESERVED, CANCELLED, FAILED
 * RESERVED    | PAID, CANCELLED, FAILED
 * PAID        | FULFILLING, CANCELLED, FAILED
 * FULFILLING  | COMPLETED, FAILED
 * COMPLETED   | (terminal)
 * CANCELLED   | (terminal)
 * FAILED      | (terminal)
 * </pre>
 *
 * Cancellation is allowed only before fulfilment starts (CREATED, PRICED, RESERVED, PAID).
 */
public final class OrderStateMachine {

  private static final Map<OrderStatus, Set<OrderStatus>> TRANSITIONS = buildTable();

  private static Map<OrderStatus, Set<OrderStatus>> buildTable() {
    Map<OrderStatus, Set<OrderStatus>> table = new EnumMap<>(OrderStatus.class);
    table.put(
        OrderStatus.CREATED,
        EnumSet.of(OrderStatus.PRICED, OrderStatus.CANCELLED, OrderStatus.FAILED));
    table.put(
        OrderStatus.PRICED,
        EnumSet.of(OrderStatus.RESERVED, OrderStatus.CANCELLED, OrderStatus.FAILED));
    table.put(
        OrderStatus.RESERVED,
        EnumSet.of(OrderStatus.PAID, OrderStatus.CANCELLED, OrderStatus.FAILED));
    table.put(
        OrderStatus.PAID,
        EnumSet.of(OrderStatus.FULFILLING, OrderStatus.CANCELLED, OrderStatus.FAILED));
    table.put(OrderStatus.FULFILLING, EnumSet.of(OrderStatus.COMPLETED, OrderStatus.FAILED));
    table.put(OrderStatus.COMPLETED, EnumSet.noneOf(OrderStatus.class));
    table.put(OrderStatus.CANCELLED, EnumSet.noneOf(OrderStatus.class));
    table.put(OrderStatus.FAILED, EnumSet.noneOf(OrderStatus.class));
    return table;
  }

  /** Returns whether {@code from -> to} is in the transition table. */
  public boolean canTransition(OrderStatus from, OrderStatus to) {
    return TRANSITIONS.get(from).contains(to);
  }

  /**
   * Validates {@code from -> to} and returns {@code to}.
   *
   * @throws IllegalTransitionException if the pair is not in the table
   */
  public OrderStatus transition(OrderStatus from, OrderStatus to) {
    if (!canTransition(from, to)) {
      throw new IllegalTransitionException(from, to);
    }
    return to;
  }
}
