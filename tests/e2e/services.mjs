// Shërbim me mjeshtër nga fillimi në fund, vetëm nga API-ja, me numrat e provës: mjeshtri aplikon,
// Operacionet e miratojnë, klienti kërkon punë, mjeshtri jep çmimin, klienti zgjedh, puna nis dhe
// mbyllet. Asnjë sekret nuk shtypet.
const base = process.env.BASE ?? 'https://dev.krejt.app';
const CODE = '111111';
const adminPhone = '+38344100200';
const customerPhone = '+38344100201';
const providerPhone = '+38344100202';

const homeAt = { lat: 42.65, lng: 21.18 };

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
      body: { phone, code: CODE, locale: 'sq', device: { id: `e2e-service-${who}`, name: 'Prova e shërbimit', platform: 'web' } },
    }),
    (r) => r.status === 200 && r.json.access_token,
  );
  return pair.access_token;
}

const admin = await login(adminPhone, 'admin');
const customer = await login(customerPhone, 'klienti');
const provider = await login(providerPhone, 'mjeshtri');

// ---- 1. kategoritë dhe mjeshtri i miratuar -----------------------------------------------------
const cats = step('klienti: kategoritë', await call('/api/v1/services/categories', { token: customer }), (r) => r.status === 200 && (r.json.items ?? []).length > 0);
const category = cats.items.find((c) => c.id === 'electrician')?.id ?? cats.items[0].id;
console.log(`     ${cats.items.length} kategori · zgjedhur ${category}`);

const applied = step(
  'mjeshtri: aplikimi',
  await call('/api/v1/services/provider', {
    method: 'POST',
    token: provider,
    body: { categories: [category], city: 'Prishtinë', business_name: 'Prova Servis' },
  }),
  (r) => r.status === 200 && r.json.user_id,
);
console.log(`     gjendja: ${applied.status}`);

// Pa miratim, mjeshtri nuk i sheh punët e hapura.
if (applied.status !== 'approved') {
  step('mjeshtri: pa miratim nuk sheh punë', await call('/api/v1/services/provider/open', { token: provider }), (r) => r.status === 403);
}
step(
  'admin: miratimi',
  await call(`/api/v1/admin/service-providers/${applied.user_id}`, { method: 'PATCH', token: admin, body: { status: 'approved' } }),
  (r) => r.status === 200 && r.json.status === 'approved',
);

// ---- 2. klienti: kërkesa ------------------------------------------------------------------------
const request = step(
  'klienti: kërkesa',
  await call('/api/v1/services/requests', {
    method: 'POST',
    token: customer,
    idem: true,
    body: {
      category_id: category,
      title: 'Priza nuk punon',
      description: 'Priza e kuzhinës nuk ka rrymë që dje; automati nuk bie.',
      address_line1: 'Rruga C, nr. 7',
      address: homeAt,
      payment_method: 'cash',
    },
  }),
  (r) => (r.status === 201 || r.status === 200) && r.json.id && r.json.state === 'open',
);
console.log(`     kërkesa ${request.id} · ${request.code}`);

// Përshkrimi shumë i shkurtër refuzohet.
step(
  'klienti: përshkrim i shkurtër refuzohet',
  await call('/api/v1/services/requests', {
    method: 'POST',
    token: customer,
    idem: true,
    body: { category_id: category, title: 'X', description: 'shkurt', address_line1: 'Rr. C', address: homeAt, payment_method: 'cash' },
  }),
  (r) => r.status === 422,
);

// ---- 3. mjeshtri: e sheh punën dhe jep çmimin ---------------------------------------------------
const open = step('mjeshtri: punët e hapura', await call('/api/v1/services/provider/open', { token: provider }), (r) => r.status === 200 && (r.json.items ?? []).some((x) => x.id === request.id));
const mine = open.items.find((x) => x.id === request.id);
step('mjeshtri: sheh qytetin, jo adresën', { status: 200, json: mine }, (r) => !!r.json.city && r.json.address_line1 === undefined);
console.log(`     qyteti: ${mine.city}`);

const offer = step(
  'mjeshtri: oferta',
  await call(`/api/v1/services/provider/requests/${request.id}/offer`, {
    method: 'POST',
    token: provider,
    body: { price_minor: 2500, note: 'Zëvendësim prize dhe kontroll i linjës.' },
  }),
  (r) => r.status === 200 && r.json.id && r.json.price_minor === 2500,
);

// ---- 4. klienti e sheh ofertën dhe zgjedh -------------------------------------------------------
const withOffers = step('klienti: ofertat', await call(`/api/v1/services/requests/${request.id}`, { token: customer }), (r) => r.status === 200 && (r.json.offers ?? []).some((o) => o.id === offer.id));
console.log(`     ${withOffers.offers.length} ofertë · ${(withOffers.offers[0].price_minor / 100).toFixed(2)}`);

const booked = step(
  'klienti: zgjedh ofertën',
  await call(`/api/v1/services/requests/${request.id}/accept`, { method: 'POST', token: customer, body: { offer_id: offer.id } }),
  (r) => r.status === 200 && r.json.state === 'booked' && r.json.price_minor === 2500,
);
console.log(`     gjendja: ${booked.state} · mjeshtri ${booked.provider?.business_name ?? '—'}`);

// ---- 5. mjeshtri: nis dhe përfundon --------------------------------------------------------------
step('mjeshtri: nis punën', await call(`/api/v1/services/provider/requests/${request.id}/start`, { method: 'POST', token: provider }), (r) => r.status === 200 && r.json.state === 'in_progress');
step('klienti: anulimi pas nisjes refuzohet', await call(`/api/v1/services/requests/${request.id}/cancel`, { method: 'POST', token: customer, body: {} }), (r) => r.status === 409);
const done = step('mjeshtri: përfundon', await call(`/api/v1/services/provider/requests/${request.id}/complete`, { method: 'POST', token: provider }), (r) => r.status === 200 && r.json.state === 'completed');
console.log(`     gjendja: ${done.state} · pagesa ${done.payment_status}`);

const final = step('klienti: puna e mbyllur', await call(`/api/v1/services/requests/${request.id}`, { token: customer }), (r) => r.status === 200 && r.json.state === 'completed');
console.log(`     çmimi ${(final.price_minor / 100).toFixed(2)}`);

console.log('U KRYE — shërbim nga kërkesa te puna e mbyllur');
