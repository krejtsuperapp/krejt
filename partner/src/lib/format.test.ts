import assert from 'node:assert/strict';
import { test } from 'node:test';

import { clock, money, orderState, orderTone, waited } from './format.ts';

test('paraja formatohet nga cent', () => {
  assert.equal(money(900), '9,00 €');
  assert.equal(money(0), '0,00 €');
  assert.equal(money(5), '0,05 €');
  assert.equal(money(-250), '-2,50 €');
});

test('pritja lexohet shkurt nga larg', () => {
  assert.equal(waited(0), 'tani');
  assert.equal(waited(7), '7 min');
  assert.equal(waited(60), '1 h');
  assert.equal(waited(95), '1 h 35 min');
});

test('çdo gjendje porosie ka tekst dhe ngjyrë', () => {
  const states = [
    'pending_merchant',
    'accepted',
    'preparing',
    'ready',
    'courier_assigned',
    'picked_up',
    'delivered',
    'cancelled',
    'rejected',
  ];
  for (const s of states) {
    assert.notEqual(orderState(s), s, s);
    assert.ok(orderTone(s).length > 0, s);
  }
});

test('porosia e re bie në sy si e kuqe, e gatshmja si e gjelbër', () => {
  assert.equal(orderTone('pending_merchant'), 'danger');
  assert.equal(orderTone('ready'), 'ok');
});

test('gjendja e panjohur kalon ashtu siç është, pa rrëzuar faqen', () => {
  assert.equal(orderState('diçka_e_re'), 'diçka_e_re');
  assert.equal(orderTone('diçka_e_re'), 'muted');
});

test('ora që mungon ose s\'lexohet shfaqet si vizë', () => {
  assert.equal(clock(null), '—');
  assert.equal(clock('jo-datë'), '—');
});
