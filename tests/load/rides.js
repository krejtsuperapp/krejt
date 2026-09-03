// KREJT — test ngarkese (§69) me k6: rrjedha e udhëtimit nga ana e klientit dhe e shoferit kundër staging-ut.
// Ekzekutimi: k6 run -e BASE=https://dev.krejt.app -e SMOKE=1 tests/load/rides.js   (dev: kyçet vetë me numrat e provës)
//             k6 run -e BASE=https://staging.krejt.app -e CUSTOMER_TOKEN=... -e DRIVER_TOKEN=... tests/load/rides.js
// Token-at merren nga OTP-ja e llogarive të testit (asnjë sekret në skedar). Pragjet: p95 quote < 500 ms,
// p95 request < 800 ms, gabime < 1 %. Skenari "peak" = 2–3× piku i pritur i Prishtinës (§69: 2×–3× e pikut).
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const BASE = __ENV.BASE || 'http://localhost:8080';
// Token-at jepen me mjedis, ose merren vetë me numrat e provës (vetëm në development).
const CODE = __ENV.TEST_OTP || '111111';
const CUSTOMER_PHONE = __ENV.CUSTOMER_PHONE || '+38344100201';
const DRIVER_PHONE = __ENV.DRIVER_PHONE || '+38344100202';
// SMOKE=1: një minutë e lehtë, për të provuar rrjedhën pa e ngarkuar dev-in.
const SMOKE = __ENV.SMOKE === '1';

const quoteTime = new Trend('quote_ms', true);
const requestTime = new Trend('ride_request_ms', true);
const errors = new Rate('errors');

const full = {
  scenarios: {
    quotes: { executor: 'ramping-arrival-rate', startRate: 5, timeUnit: '1s', preAllocatedVUs: 50, maxVUs: 300,
      stages: [{ target: 30, duration: '2m' }, { target: 90, duration: '5m' }, { target: 0, duration: '1m' }], exec: 'quote' },
    driverLocation: { executor: 'constant-arrival-rate', rate: 200, timeUnit: '1s', duration: '8m', preAllocatedVUs: 100, maxVUs: 400, exec: 'location' },
    rideFlow: { executor: 'per-vu-iterations', vus: 20, iterations: 10, exec: 'rideFlow', startTime: '30s' },
  },
  thresholds: {
    quote_ms: ['p(95)<500'],
    ride_request_ms: ['p(95)<800'],
    errors: ['rate<0.01'],
    http_req_failed: ['rate<0.01'],
  },
};

const smoke = {
  scenarios: {
    quotes: { executor: 'constant-arrival-rate', rate: 3, timeUnit: '1s', duration: '40s', preAllocatedVUs: 5, maxVUs: 20, exec: 'quote' },
    driverLocation: { executor: 'constant-arrival-rate', rate: 5, timeUnit: '1s', duration: '40s', preAllocatedVUs: 5, maxVUs: 20, exec: 'location' },
    rideFlow: { executor: 'per-vu-iterations', vus: 2, iterations: 2, exec: 'rideFlow', startTime: '5s' },
  },
  thresholds: full.thresholds,
};

export const options = SMOKE ? smoke : full;

function login(phone, appId) {
  const h = { 'Content-Type': 'application/json', 'X-App-Id': appId, 'X-App-Platform': 'android', 'X-App-Version': '1.0.0' };
  http.post(`${BASE}/api/v1/auth/otp/request`, JSON.stringify({ phone, locale: 'sq' }), { headers: h });
  const v = http.post(`${BASE}/api/v1/auth/otp/verify`,
    JSON.stringify({ phone, code: CODE, locale: 'sq', device: { id: `k6-${appId}`, name: 'k6', platform: 'android' } }), { headers: h });
  if (v.status !== 200) throw new Error(`kyçja e ${appId} dështoi: ${v.status} ${v.body}`);
  return v.json('access_token');
}

// Ekzekutohet një herë para skenarëve; rezultati u kalohet funksioneve si `data`.
export function setup() {
  const customer = __ENV.CUSTOMER_TOKEN || login(CUSTOMER_PHONE, 'customer');
  const driver = __ENV.DRIVER_TOKEN || login(DRIVER_PHONE, 'driver');
  // Shoferi duhet të jetë në punë që pozicionet të pranohen.
  http.post(`${BASE}/api/v1/driver/online`, JSON.stringify({ categories: ['economy'] }), { headers: headers(driver) });
  return { customer, driver };
}

export function teardown(data) {
  http.post(`${BASE}/api/v1/driver/offline`, null, { headers: headers(data.driver) });
}

const headers = (token) => ({ 'Content-Type': 'application/json', Authorization: `Bearer ${token}`,
  'X-App-Id': 'customer', 'X-App-Platform': 'android', 'X-App-Version': '1.0.0' });

// pika brenda Prishtinës (rreze ~3 km nga qendra)
function point() {
  return { lat: 42.6629 + (Math.random() - 0.5) * 0.05, lng: 21.1655 + (Math.random() - 0.5) * 0.07 };
}

export function quote(data) {
  const CUSTOMER = data.customer;
  const res = http.post(`${BASE}/api/v1/rides/quote`, JSON.stringify({ pickup: point(), dropoff: point() }), { headers: headers(CUSTOMER) });
  quoteTime.add(res.timings.duration);
  const ok = check(res, { 'quote 200': (r) => r.status === 200 });
  errors.add(!ok);
  sleep(0.2);
}

export function location(data) {
  const DRIVER = data.driver;
  const p = point();
  const res = http.post(`${BASE}/api/v1/driver/location`, JSON.stringify({ samples: [{ lat: p.lat, lng: p.lng, ts: Date.now() }] }),
    { headers: { ...headers(DRIVER), 'X-App-Id': 'driver' } });
  const ok = check(res, { 'location 200/409': (r) => r.status === 200 || r.status === 409 });
  errors.add(!ok);
}

export function rideFlow(data) {
  const CUSTOMER = data.customer;
  const q = http.post(`${BASE}/api/v1/rides/quote`, JSON.stringify({ pickup: point(), dropoff: point() }), { headers: headers(CUSTOMER) });
  if (q.status !== 200) { errors.add(1); return; }
  const quoteId = q.json('quotes.0.id');
  const t0 = Date.now();
  const r = http.post(`${BASE}/api/v1/rides`, JSON.stringify({ quote_id: quoteId, payment_method: 'cash' }),
    { headers: { ...headers(CUSTOMER), 'Idempotency-Key': `k6-${__VU}-${__ITER}-${Date.now()}` } });
  requestTime.add(Date.now() - t0);
  const created = check(r, { 'ride 201 ose 409 aktiv': (x) => x.status === 201 || x.status === 409 });
  errors.add(!created);
  if (r.status === 201) {
    const id = r.json('id');
    sleep(2);
    http.post(`${BASE}/api/v1/rides/${id}/cancel`, JSON.stringify({ reason: 'k6' }), { headers: headers(CUSTOMER) });
  }
  sleep(1);
}
