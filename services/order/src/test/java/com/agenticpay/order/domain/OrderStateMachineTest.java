package com.agenticpay.order.domain;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.util.EnumSet;
import java.util.Map;
import java.util.Set;
import org.junit.jupiter.api.Test;

class OrderStateMachineTest {

  private static final Set<OrderStatus> TERMINAL =
      EnumSet.of(OrderStatus.COMPLETED, OrderStatus.CANCELLED, OrderStatus.FAILED);

  /** Independent copy of the table in the ticket/README; deliberately not shared with main code. */
  private static final Map<OrderStatus, Set<OrderStatus>> LEGAL =
      Map.of(
          OrderStatus.CREATED,
          EnumSet.of(OrderStatus.PRICED, OrderStatus.CANCELLED, OrderStatus.FAILED),
          OrderStatus.PRICED,
          EnumSet.of(OrderStatus.RESERVED, OrderStatus.CANCELLED, OrderStatus.FAILED),
          OrderStatus.RESERVED,
          EnumSet.of(OrderStatus.PAID, OrderStatus.CANCELLED, OrderStatus.FAILED),
          OrderStatus.PAID,
          EnumSet.of(OrderStatus.FULFILLING, OrderStatus.CANCELLED, OrderStatus.FAILED),
          OrderStatus.FULFILLING,
          EnumSet.of(OrderStatus.COMPLETED, OrderStatus.FAILED),
          OrderStatus.COMPLETED,
          EnumSet.noneOf(OrderStatus.class),
          OrderStatus.CANCELLED,
          EnumSet.noneOf(OrderStatus.class),
          OrderStatus.FAILED,
          EnumSet.noneOf(OrderStatus.class));

  private final OrderStateMachine machine = new OrderStateMachine();

  @Test
  void allows_every_legal_transition_and_rejects_every_other_pair() {
    for (OrderStatus from : OrderStatus.values()) {
      for (OrderStatus to : OrderStatus.values()) {
        if (LEGAL.get(from).contains(to)) {
          assertThat(machine.transition(from, to)).as("%s -> %s", from, to).isEqualTo(to);
          assertThat(machine.canTransition(from, to)).isTrue();
        } else {
          assertThatThrownBy(() -> machine.transition(from, to))
              .as("%s -> %s", from, to)
              .isInstanceOf(IllegalTransitionException.class);
          assertThat(machine.canTransition(from, to)).isFalse();
        }
      }
    }
  }

  @Test
  void exception_reports_the_rejected_pair() {
    assertThatThrownBy(() -> machine.transition(OrderStatus.CREATED, OrderStatus.PAID))
        .isInstanceOfSatisfying(
            IllegalTransitionException.class,
            e -> {
              assertThat(e.from()).isEqualTo(OrderStatus.CREATED);
              assertThat(e.to()).isEqualTo(OrderStatus.PAID);
            });
  }

  @Test
  void terminal_states_allow_no_outgoing_transitions() {
    for (OrderStatus from : TERMINAL) {
      for (OrderStatus to : OrderStatus.values()) {
        assertThatThrownBy(() -> machine.transition(from, to))
            .as("%s -> %s", from, to)
            .isInstanceOf(IllegalTransitionException.class);
      }
    }
  }

  @Test
  void cancel_is_allowed_only_before_fulfilment() {
    for (OrderStatus from :
        EnumSet.of(
            OrderStatus.CREATED, OrderStatus.PRICED, OrderStatus.RESERVED, OrderStatus.PAID)) {
      assertThat(machine.canTransition(from, OrderStatus.CANCELLED)).as("%s", from).isTrue();
    }
    for (OrderStatus from : EnumSet.of(OrderStatus.FULFILLING, OrderStatus.COMPLETED)) {
      assertThatThrownBy(() -> machine.transition(from, OrderStatus.CANCELLED))
          .as("%s", from)
          .isInstanceOf(IllegalTransitionException.class);
    }
  }
}
