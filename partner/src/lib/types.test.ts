import assert from 'node:assert/strict';
import { test } from 'node:test';

import { isOpen, nextStep, type Order, waitedMinutes } from './types.ts';

const order = (over: Partial<Order> = {}): Order => ({
  id: 'o1',
  code: 'K7F3QA',
  state: 'pending_merchant',
  fulfillment: 'courier',
  payment_method: 'cash',
  payment_status: 'none',
  items_total_minor: 800,
  delivery_fee_minor: 100,
  total_minor: 900,
  currency: 'EUR',
  note: null,
  prep_time_min: 20,
  ready_at_estimate: null,
  created_at: '2026-09-02T12:00:00Z',
  accepted_at: null,
  items: [],
  ...over,
});

test('radha mban vetëm porositë ku kuzhina ka ende punë', () => {
  for (const state of ['pending_merchant', 'accepted', 'preparing', 'ready']) {
    assert.ok(isOpen(order({ state })), state);
  }
  for (const state of ['courier_assigned', 'picked_up', 'delivered', 'cancelled', 'rejected']) {
    assert.ok(!isOpen(order({ state })), state);
  }
});

test('hapi i radhës ndjek rrjedhën e kuzhinës', () => {
  assert.equal(nextStep(order({ state: 'pending_merchant' }))?.to, 'accepted');
  assert.equal(nextStep(order({ state: 'accepted' }))?.to, 'preparing');
  assert.equal(nextStep(order({ state: 'preparing' }))?.to, 'ready');
});

test('dorëzimin e mbyll vendi vetëm kur nuk ka korrier në mes', () => {
  assert.equal(nextStep(order({ state: 'ready', fulfillment: 'courier' })), null);
  assert.equal(nextStep(order({ state: 'ready', fulfillment: 'pickup' }))?.to, 'delivered');
  assert.equal(
    nextStep(order({ state: 'ready', fulfillment: 'merchant_delivers' }))?.to,
    'delivered',
  );
});

test('porositë e mbyllura nuk kanë hap tjetër', () => {
  for (const state of ['delivered', 'cancelled', 'rejected', 'picked_up']) {
    assert.equal(nextStep(order({ state })), null, state);
  }
});

test('pritja matet nga pranimi kur ka ndodhur, ndryshe nga krijimi', () => {
  const now = new Date('2026-09-02T12:30:00Z');
  assert.equal(waitedMinutes(order(), now), 30);
  assert.equal(
    waitedMinutes(order({ accepted_at: '2026-09-02T12:20:00Z' }), now),
    10,
  );
});

test('një orë e ardhme nuk jep pritje negative', () => {
  const now = new Date('2026-09-02T11:00:00Z');
  assert.equal(waitedMinutes(order(), now), 0);
});
