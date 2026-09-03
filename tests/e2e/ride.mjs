// Udhëtim i plotë vetëm nga API-ja, me numrat e provës: klienti porosit, shoferi pranon,
// mbërrin, nis me kodin e marrjes, përfundon; klienti vlerëson. Asnjë sekret nuk shtypet.
const base = process.env.BASE ?? 'https://dev.krejt.app';
const CODE = '111111';
const customerPhone = '+38344100201';
const driverPhone = '+38344100202';

// Prishtinë: qendra → Sunny Hill, brenda zonës së shërbimit.
const pickup = { lat: 42.6629, lng: 21.1655 };
const dropoff = { lat: 42.65, lng: 21.18 };

let failed = false;
const uuid = () => crypto.randomUUID();

async function call(path, { method = 'GET', body, token, idem = false } = {}) {
  const res = await fetch(base + path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(idem ? { 'Idempotency-Key': uuid() } : {}),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  let json;
  try {
    json = JSON.parse(text);
  } catch {
    json = { raw: text.slice(0, 200) };
  }
  return { status: res.status, json };
}

const step = (name, r, ok) => {
  const good = ok(r);
  console.log(`${good ? 'OK ' : 'DËSHTOI'}  ${name} → ${r.status}${good ? '' : ' ' + JSON.stringify(r.json).slice(0, 240)}`);
  if (!good) {
    failed = true;
    process.exit(1);
  }
  return r.json;
};

async function login(phone, who) {
  await call('/api/v1/auth/otp/request', { method: 'POST', body: { phone, locale: 'sq' } });
  const pair = step(
    `${who}: kyçja`,
    await call('/api/v1/auth/otp/verify', {
      method: 'POST',
      body: { phone, code: CODE, locale: 'sq', device: { id: `e2e-ride-${who}`, name: 'Prova e udhëtimit', platform: 'web' } },
    }),
    (r) => r.status === 200 && r.json.access_token,
  );
  return pair.access_token;
}

const wait = (ms) => new Promise((r) => setTimeout(r, ms));

const customer = await login(customerPhone, 'klienti');
const driver = await login(driverPhone, 'shoferi');

// Pastrim: nëse shoferi ka mbetur me një udhëtim aktiv nga një provë e mëparshme, anulohet.
const stale = await call('/api/v1/driver/rides/active', { token: driver });
if (stale.status === 200 && stale.json && stale.json.id) {
  const c = await call(`/api/v1/driver/rides/${stale.json.id}/cancel`, { method: 'POST', token: driver, body: { reason: 'prova e mëparshme' } });
  console.log(`     pastrim: udhëtimi i vjetër ${stale.json.id.slice(0, 8)} → ${c.status}`);
}

// Pastrim edhe te klienti: një porosi e mbetur nga një provë e mëparshme bllokon të renë (409).
{
  const mine = await call('/api/v1/rides', { token: customer });
  const finished = new Set(['completed', 'cancelled', 'no_driver']);
  for (const r of (mine.json.items ?? [])) {
    if (!finished.has(r.state)) {
      const c = await call(`/api/v1/rides/${r.id}/cancel`, { method: 'POST', token: customer, body: { reason: 'prova e mëparshme' } });
      console.log(`     pastrim: klienti anuloi ${r.id.slice(0, 8)} (${r.state}) → ${c.status}`);
    }
  }
}

// 1. shoferi del në punë dhe dërgon pozicionin afër marrjes
step('shoferi: në punë', await call('/api/v1/driver/online', { method: 'POST', token: driver, body: { categories: ['economy', 'comfort'] } }), (r) => r.status < 300);
step(
  'shoferi: pozicioni',
  await call('/api/v1/driver/location', { method: 'POST', token: driver, body: { samples: [{ lat: 42.6612, lng: 21.1631, ts: Date.now() }] } }),
  (r) => r.status < 300,
);

// 2. klienti merr çmimin dhe porosit
const quote = step(
  'klienti: çmimi',
  await call('/api/v1/rides/quote', { method: 'POST', token: customer, body: { pickup, dropoff, pickup_address: 'Sheshi Nëna Terezë', dropoff_address: 'Sunny Hill' } }),
  (r) => r.status === 200 && Array.isArray(r.json.quotes) && r.json.quotes.length > 0,
);
const q = quote.quotes[0];
console.log(`     ${quote.distance_m} m · ${quote.duration_s} s · ${q.category} ${(q.price_minor / 100).toFixed(2)} ${q.currency}`);
const ride = step(
  'klienti: porosia',
  await call('/api/v1/rides', { method: 'POST', token: customer, body: { quote_id: q.id, payment_method: 'cash' }, idem: true }),
  (r) => r.status === 201 || r.status === 200,
);
console.log(`     udhëtimi ${ride.id} · ${ride.state}`);

// 3. oferta mbërrin te shoferi (dispeçeri punon me radhë; pritet deri 30 s)
let offer = null;
for (let i = 0; i < 15 && !offer; i++) {
  await wait(2000);
  const o = await call('/api/v1/driver/offers', { token: driver });
  offer = (o.json.items ?? []).find((x) => x.ride_id === ride.id) ?? null;
}
step('shoferi: oferta', { status: offer ? 200 : 404, json: offer ?? { info: 'asnjë ofertë për këtë udhëtim brenda 30 s' } }, () => !!offer);
console.log(`     oferta ${offer.id.slice(0, 8)} · fitimi ${(offer.earnings_minor / 100).toFixed(2)}`);

// 4. pranimi → caktimi
const assigned = step('shoferi: pranimi', await call(`/api/v1/driver/offers/${offer.id}/accept`, { method: 'POST', token: driver, idem: true }), (r) => r.status === 200);
console.log(`     gjendja: ${assigned.state}`);

// 5. mbërritja → klienti sheh kodin e marrjes
step('shoferi: mbërrita', await call(`/api/v1/driver/rides/${ride.id}/arrived`, { method: 'POST', token: driver }), (r) => r.status === 200);
const seen = step('klienti: udhëtimi', await call(`/api/v1/rides/${ride.id}`, { token: customer }), (r) => r.status === 200 && r.json.pickup_code);
console.log(`     gjendja: ${seen.state} · shoferi: ${seen.driver?.name ?? '—'} ${seen.driver?.vehicle_plate ?? ''}`);

// 6. kodi i gabuar refuzohet; i sakti e nis udhëtimin
// Refuzim si kod i gabuar, jo si formë e gabuar: VALIDATION_FAILED do të thoshte kontratë e thyer.
step('shoferi: kod i gabuar refuzohet', await call(`/api/v1/driver/rides/${ride.id}/start`, { method: 'POST', token: driver, body: { code: '0000' } }), (r) => r.status >= 400 && r.json?.error?.code !== 'VALIDATION_FAILED');
const started = step('shoferi: nisja me kodin', await call(`/api/v1/driver/rides/${ride.id}/start`, { method: 'POST', token: driver, body: { code: seen.pickup_code } }), (r) => r.status === 200);
console.log(`     gjendja: ${started.state}`);

// 7. përfundimi dhe vlerësimi
const done = step('shoferi: përfundimi', await call(`/api/v1/driver/rides/${ride.id}/complete`, { method: 'POST', token: driver, idem: true }), (r) => r.status === 200);
console.log(`     gjendja: ${done.state} · çmimi final ${done.price_final_minor != null ? (done.price_final_minor / 100).toFixed(2) : '—'} · pagesa ${done.payment_status}`);
step('klienti: vlerësimi', await call(`/api/v1/rides/${ride.id}/review`, { method: 'POST', token: customer, body: { rating: 5, tags: ['friendly', 'clean_car'] } }), (r) => r.status < 300);

// 8. shoferi del nga puna
step('shoferi: jashtë pune', await call('/api/v1/driver/offline', { method: 'POST', token: driver }), (r) => r.status < 300);

console.log('U KRYE — udhëtim i plotë nga porosia te vlerësimi');
