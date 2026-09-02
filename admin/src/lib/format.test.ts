import assert from 'node:assert/strict';
import { test } from 'node:test';

import { ago, distance, duration, money, rideState, rideTone, shortId } from './format.ts';

test('paraja formatohet nga cent, me minusin para simbolit', () => {
  assert.equal(money(1240), '12,40 €');
  assert.equal(money(0), '0,00 €');
  assert.equal(money(5), '0,05 €');
  assert.equal(money(-750), '-7,50 €');
});

test('distanca kalon në kilometra vetëm mbi 950 m', () => {
  assert.equal(distance(940), '940 m');
  assert.equal(distance(1500), '1,5 km');
  assert.equal(distance(24300), '24 km');
});

test('kohëzgjatja lexohet si njeri', () => {
  assert.equal(duration(30), '< 1 min');
  assert.equal(duration(420), '7 min');
  assert.equal(duration(3600), '1 h');
  assert.equal(duration(5400), '1 h 30 min');
});

test('koha e kaluar matet nga çasti i dhënë', () => {
  const now = new Date('2026-09-02T12:00:00Z');
  assert.equal(ago('2026-09-02T11:59:55Z', now), 'tani');
  assert.equal(ago('2026-09-02T11:59:30Z', now), '30 s');
  assert.equal(ago('2026-09-02T11:45:00Z', now), '15 min');
  assert.equal(ago('2026-09-02T09:00:00Z', now), '3 h');
  assert.equal(ago('2026-08-30T12:00:00Z', now), '3 d');
});

test('data që mungon ose s\'lexohet nuk rrëzon faqen', () => {
  assert.equal(ago(null), '—');
  assert.equal(ago('jo-datë'), '—');
});

test('çdo gjendje udhëtimi ka tekst dhe ngjyrë; e panjohura kalon ashtu siç është', () => {
  for (const s of ['matching', 'assigned', 'arrived', 'in_progress', 'completed', 'cancelled', 'no_driver']) {
    assert.notEqual(rideState(s), s);
    assert.ok(rideTone(s).length > 0);
  }
  assert.equal(rideState('diçka_e_re'), 'diçka_e_re');
  assert.equal(rideTone('diçka_e_re'), 'muted');
});

test('identifikuesit shkurtohen vetëm kur janë të gjatë', () => {
  assert.equal(shortId('abc'), 'abc');
  assert.equal(shortId('0123456789abcdef'), '01234567');
});
