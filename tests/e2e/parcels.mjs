// Dërgesë pakoje nga fillimi në fund, vetëm nga API-ja, me numrat e provës: klienti merr çmimin dhe
// kërkon pakon, korrieri hyn në punë, pranon ofertën, e merr me kodin e marrjes dhe e dorëzon me
// kodin e dorëzimit; kodet e gabuara refuzohen. Asnjë sekret nuk shtypet.
const base = process.env.BASE ?? 'https://dev.krejt.app';
const CODE = '111111';
const customerPhone = '+38344100201';
const courierPhone = '+38344100202';

const pickupAt = { lat: 42.6612, lng: 21.1631 };
const dropoffAt = { lat: 42.65, lng: 21.18 };

const uuid = () => crypto.randomUUID();
const wait = (ms) => new Promise((r) => setTimeout(r, ms));

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
  console.log(`${good ? 'OK ' : 'DËSHTOI'}  ${name} → ${r.status}${good ? '' : ' ' + JSON.stringify(r.json).slice(0, 260)}`);
  if (!good) process.exit(1);
  return r.json;
};

async function login(phone, who) {
  await call('/api/v1/auth/otp/request', { method: 'POST', body: { phone, locale: 'sq' } });
  const pair = step(
    `${who}: kyçja`,
    await call('/api/v1/auth/otp/verify', {
      method: 'POST',
      body: { phone, code: CODE, locale: 'sq', device: { id: `e2e-parcel-${who}`, name: 'Prova e pakos', platform: 'web' } },
    }),
    (r) => r.status === 200 && r.json.access_token,
  );
  return pair.access_token;
}

const customer = await login(customerPhone, 'klienti');
const courier = await login(courierPhone, 'kurieri');

// ---- 1. kurieri në punë, afër pikës së marrjes ------------------------------------------------
step('kurieri: në punë', await call('/api/v1/driver/online', { method: 'POST', token: courier, body: { categories: ['economy'] } }), (r) => r.status < 300);
step(
  'kurieri: pozicioni',
  await call('/api/v1/driver/location', { method: 'POST', token: courier, body: { samples: [{ lat: pickupAt.lat, lng: pickupAt.lng, ts: Date.now() }] } }),
  (r) => r.status < 300,
);

// ---- 2. klienti: kërkimi i adresës, çmimi dhe kërkesa -----------------------------------------
const places = step('klienti: kërkimi i vendit', await call('/api/v1/places/search?q=Prishtin&lat=42.66&lng=21.16', { token: customer }), (r) => r.status === 200 && Array.isArray(r.json.items));
console.log(`     ${places.items.length} vende për "Prishtin"`);

const quote = step(
  'klienti: çmimi',
  await call('/api/v1/parcels/quote', {
    method: 'POST',
    token: customer,
    body: { size: 'm', pickup: pickupAt, pickup_address: 'Rruga B, nr. 1', dropoff: dropoffAt, dropoff_address: 'Rruga C, nr. 7' },
  }),
  (r) => r.status === 200 && r.json.id && r.json.price_minor > 0,
);
console.log(`     ${(quote.price_minor / 100).toFixed(2)} ${quote.currency} · ${quote.distance_m} m`);

const parcel = step(
  'klienti: kërkesa',
  await call('/api/v1/parcels', {
    method: 'POST',
    token: customer,
    idem: true,
    body: { quote_id: quote.id, payment_method: 'cash', recipient_name: 'Marrësi i provës', recipient_phone: '+38344100203', note: 'Prova e2e' },
  }),
  (r) => (r.status === 201 || r.status === 200) && r.json.id && r.json.pickup_code && r.json.delivery_code,
);
console.log(`     pakoja ${parcel.id} · ${parcel.state} · kodi ${parcel.code}`);

step('klienti: pakoja aktive', await call('/api/v1/parcels/active', { token: customer }), (r) => r.status === 200 && r.json.parcel?.id === parcel.id);

// ---- 3. kurieri: oferta → pranimi → marrja → dorëzimi ------------------------------------------
let offer = null;
for (let i = 0; i < 15 && !offer; i++) {
  await wait(2000);
  const o = await call('/api/v1/courier/parcel-offers', { token: courier });
  offer = (o.json.items ?? []).find((x) => x.parcel_id === parcel.id) ?? null;
}
step('kurieri: oferta', { status: offer ? 200 : 404, json: offer ?? { info: 'asnjë ofertë brenda 30 s' } }, () => !!offer);
const assigned = step('kurieri: pranimi', await call(`/api/v1/courier/parcel-offers/${offer.id}/accept`, { method: 'POST', token: courier, idem: true }), (r) => r.status === 200 && r.json.state === 'courier_assigned');
step('kurieri: kodet nuk i sheh', { status: 200, json: assigned }, (r) => !r.json.pickup_code && !r.json.delivery_code);
step('kurieri: kod marrjeje i gabuar refuzohet', await call(`/api/v1/courier/parcels/${parcel.id}/pickup`, { method: 'POST', token: courier, body: { code: '0000' === parcel.pickup_code ? '1111' : '0000' } }), (r) => r.status === 422);
const picked = step('kurieri: marrja me kodin', await call(`/api/v1/courier/parcels/${parcel.id}/pickup`, { method: 'POST', token: courier, body: { code: parcel.pickup_code } }), (r) => r.status === 200 && r.json.state === 'picked_up');
console.log(`     gjendja: ${picked.state}`);
step('klienti: anulimi pas marrjes refuzohet', await call(`/api/v1/parcels/${parcel.id}/cancel`, { method: 'POST', token: customer, body: {} }), (r) => r.status === 409);
step('kurieri: kod dorëzimi i gabuar refuzohet', await call(`/api/v1/courier/parcels/${parcel.id}/deliver`, { method: 'POST', token: courier, body: { code: '0000' === parcel.delivery_code ? '1111' : '0000' } }), (r) => r.status === 422);
const delivered = step('kurieri: dorëzimi me kodin', await call(`/api/v1/courier/parcels/${parcel.id}/deliver`, { method: 'POST', token: courier, body: { code: parcel.delivery_code } }), (r) => r.status === 200 && r.json.state === 'delivered');
console.log(`     gjendja: ${delivered.state} · pagesa ${delivered.payment_status}`);

// ---- 4. klienti e sheh të dorëzuar --------------------------------------------------------------
const final = step('klienti: pakoja e dorëzuar', await call(`/api/v1/parcels/${parcel.id}`, { token: customer }), (r) => r.status === 200 && r.json.state === 'delivered' && r.json.courier?.vehicle_plate);
console.log(`     çmimi ${(final.price_minor / 100).toFixed(2)} · korrieri ${final.courier.name || '—'}`);

step('kurieri: jashtë pune', await call('/api/v1/driver/offline', { method: 'POST', token: courier }), (r) => r.status < 300);
console.log('U KRYE — pako nga çmimi te dorëzimi');
