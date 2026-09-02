// KREJT — test ngarkese (§69) me k6: rrjedha e udhëtimit nga ana e klientit dhe e shoferit kundër staging-ut.
// Ekzekutimi: k6 run -e BASE=https://api-staging.krejt.app -e CUSTOMER_TOKEN=... -e DRIVER_TOKEN=... tests/load/rides.js
// Token-at merren nga OTP-ja e llogarive të testit (asnjë sekret në skedar). Pragjet: p95 quote < 500 ms,
// p95 request < 800 ms, gabime < 1 %. Skenari "peak" = 2–3× piku i pritur i Prishtinës (§69: 2×–3× e pikut).
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Trend, Rate } from 'k6/metrics';

const BASE = __ENV.BASE || 'http://localhost:8080';
const CUSTOMER = __ENV.CUSTOMER_TOKEN;
const DRIVER = __ENV.DRIVER_TOKEN;

const quoteTime = new Trend('quote_ms', true);
const requestTime = new Trend('ride_request_ms', true);
const errors = new Rate('errors');

export const options = {
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

const headers = (token) => ({ 'Content-Type': 'application/json', Authorization: `Bearer ${token}`,
  'X-App-Id': 'customer', 'X-App-Platform': 'android', 'X-App-Version': '1.0.0' });

// pika brenda Prishtinës (rreze ~3 km nga qendra)
function point() {
  return { lat: 42.6629 + (Math.random() - 0.5) * 0.05, lng: 21.1655 + (Math.random() - 0.5) * 0.07 };
}

export function quote() {
  const res = http.post(`${BASE}/api/v1/rides/quote`, JSON.stringify({ pickup: point(), dropoff: point() }), { headers: headers(CUSTOMER) });
  quoteTime.add(res.timings.duration);
  const ok = check(res, { 'quote 200': (r) => r.status === 200 });
  errors.add(!ok);
  sleep(0.2);
}

export function location() {
  const p = point();
  const res = http.post(`${BASE}/api/v1/driver/location`, JSON.stringify({ samples: [{ lat: p.lat, lng: p.lng, ts: Date.now() }] }),
    { headers: { ...headers(DRIVER), 'X-App-Id': 'driver' } });
  const ok = check(res, { 'location 200/409': (r) => r.status === 200 || r.status === 409 });
  errors.add(!ok);
}

export function rideFlow() {
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
