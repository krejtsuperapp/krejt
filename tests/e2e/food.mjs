// Porosi ushqimi nga fillimi në fund, vetëm nga API-ja, me numrat e provës: tregtari
// regjistrohet dhe aktivizohet, menyja krijohet, klienti porosit, tregtari pranon → përgatit →
// gati, kurieri pranon → merr → dorëzon. Asnjë sekret nuk shtypet.
const base = process.env.BASE ?? 'https://dev.krejt.app';
const CODE = '111111';
const adminPhone = '+38344100200';
const customerPhone = '+38344100201'; // edhe pronar i tregtarit të provës
const courierPhone = '+38344100202';

const merchantAt = { lat: 42.6612, lng: 21.1631 };
const homeAt = { lat: 42.65, lng: 21.18 };

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
      body: { phone, code: CODE, locale: 'sq', device: { id: `e2e-food-${who}`, name: 'Prova e porosisë', platform: 'web' } },
    }),
    (r) => r.status === 200 && r.json.access_token,
  );
  return pair.access_token;
}

const admin = await login(adminPhone, 'admin');
const owner = await login(customerPhone, 'klienti/pronari');
const courier = await login(courierPhone, 'kurieri');

// ---- 1. tregtari i provës: ekziston, ose regjistrohet dhe aktivizohet ------------------------
let merchant = null;
{
  const mine = await call('/api/v1/merchant/mine', { token: owner });
  const list = Array.isArray(mine.json) ? mine.json : (mine.json.items ?? (mine.json.id ? [mine.json] : []));
  merchant = list.find((m) => m.name === 'Prova Grill') ?? list[0] ?? null;
}
if (!merchant) {
  merchant = step(
    'tregtari: aplikimi',
    await call('/api/v1/merchant/apply', {
      method: 'POST',
      token: owner,
      body: {
        type: 'restaurant',
        name: 'Prova Grill',
        description: 'Tregtar prove për porositë nga fillimi në fund.',
        phone: customerPhone,
        address_line1: 'Rruga B, nr. 1',
        city: 'Prishtinë',
        location: merchantAt,
        cuisines: ['grill'],
        fulfillment_mode: 'courier',
        min_order_minor: 300,
      },
    }),
    (r) => r.status === 201 || r.status === 200,
  );
}
console.log(`     tregtari ${merchant.id} · ${merchant.status}`);

if (merchant.status !== 'active') {
  merchant = step(
    'admin: aktivizimi',
    await call(`/api/v1/admin/merchants/${merchant.id}`, { method: 'PATCH', token: admin, body: { action: 'activate' } }),
    (r) => r.status === 200,
  );
}

// Orari: hapur gjithë javën, që open_now të jetë gjithmonë true për provën.
step(
  'tregtari: orari',
  await call(`/api/v1/merchant/${merchant.id}/hours`, {
    method: 'PUT',
    token: owner,
    body: { hours: [0, 1, 2, 3, 4, 5, 6].map((d) => ({ weekday: d, opens: '00:00', closes: '23:59' })) },
  }),
  (r) => r.status < 300,
);
step(
  'tregtari: pranon porosi',
  await call(`/api/v1/merchant/${merchant.id}`, { method: 'PATCH', token: owner, body: { accepting_orders: true } }),
  (r) => r.status < 300,
);

// ---- 2. menyja: një kategori dhe një produkt, nëse mungojnë ----------------------------------
let product = null;
{
  const menu = await call(`/api/v1/merchant/${merchant.id}/menu`, { token: owner });
  const cats = menu.json.categories ?? [];
  const products = menu.json.products ?? cats.flatMap((c) => c.products ?? []);
  product = products.find((p) => p.name === 'Qebapa 10') ?? products[0] ?? null;
  if (!product) {
    const cat = cats[0] ?? step(
      'menyja: kategoria',
      await call(`/api/v1/merchant/${merchant.id}/categories`, { method: 'POST', token: owner, body: { name: 'Kryesore', sort: 1, active: true } }),
      (r) => r.status === 201 || r.status === 200,
    );
    product = step(
      'menyja: produkti',
      await call(`/api/v1/merchant/${merchant.id}/products`, {
        method: 'POST',
        token: owner,
        body: { category_id: cat.id, name: 'Qebapa 10', description: 'Dhjetë qebapa me somun', price_minor: 450, unit: 'portion', available: true, tags: [], sort: 1 },
      }),
      (r) => r.status === 201 || r.status === 200,
    );
  }
}
console.log(`     produkti ${product.id} · ${(product.price_minor / 100).toFixed(2)}`);

// ---- 2b. imazhet publike: logoja e tregtarit dhe imazhi i produktit (S3 + CloudFront) --------
// Një PNG 1×1 i vërtetë: serveri verifikon llojin dhe madhësinë pas ngarkimit.
const png = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==',
  'base64',
);
async function uploadImage(kind, targetId, who) {
  const signed = step(
    `${who}: URL e nënshkruar (${kind})`,
    await call('/api/v1/media/upload-url', { method: 'POST', token: owner, body: { kind, target_id: targetId, content_type: 'image/png', size_bytes: png.length } }),
    (r) => r.status === 200 && r.json.object_key && r.json.upload?.url,
  );
  const put = await fetch(signed.upload.url, {
    method: signed.upload.method || 'PUT',
    headers: { ...(signed.upload.headers ?? {}), 'Content-Type': 'image/png' },
    body: png,
  });
  step(`${who}: PUT te bucket-i`, { status: put.status, json: {} }, (r) => r.status < 300);
  return step(
    `${who}: konfirmimi (${kind})`,
    await call('/api/v1/media', { method: 'POST', token: owner, body: { object_key: signed.object_key } }),
    (r) => r.status === 201 && r.json.object_key === signed.object_key,
  );
}
const logo = await uploadImage('merchant_logo', merchant.id, 'tregtari');
await uploadImage('product_image', product.id, 'tregtari');
{
  const pub = step('publiku: tregtari me logo', await call(`/api/v1/merchants/${merchant.slug}`), (r) => r.status === 200);
  const hasUrl = typeof pub.logo_url === 'string' && pub.logo_url.endsWith(logo.object_key);
  step('publiku: logo_url e vendosur', { status: hasUrl ? 200 : 500, json: { logo_url: pub.logo_url ?? null } }, () => hasUrl);
  if (logo.url) {
    const img = await fetch(logo.url);
    step('CloudFront: imazhi lexohet', { status: img.status, json: { type: img.headers.get('content-type') } }, (r) => r.status === 200);
  } else {
    console.log('     (MEDIA_BASE_URL ende bosh në server: URL-ja publike nuk u provua)');
  }
}

// Pastrim: një porosi aktive e mbetur te kurieri (nga një provë e ndërprerë) e përjashton atë
// nga ofertat e reja; lirohet para se të fillojë prova.
{
  const active = await call('/api/v1/courier/orders/active', { token: courier });
  const stale = active.json && active.json.id ? active.json : active.json?.order;
  if (stale && stale.id) {
    const c = await call(`/api/v1/courier/orders/${stale.id}/release`, { method: 'POST', token: courier, body: { reason: 'prova e mëparshme' } });
    console.log(`     pastrim: kurieri liroi ${stale.id.slice(0, 8)} (${stale.state}) → ${c.status}`);
  }
}
// ---- 3. kurieri në punë afër tregtarit (dispeçeri i porosive kërkon te kategoria economy) ----
step('kurieri: në punë', await call('/api/v1/driver/online', { method: 'POST', token: courier, body: { categories: ['economy', 'comfort'] } }), (r) => r.status < 300);
step('kurieri: pozicioni', await call('/api/v1/driver/location', { method: 'POST', token: courier, body: { samples: [{ lat: 42.6605, lng: 21.1625, ts: Date.now() }] } }), (r) => r.status < 300);

// ---- 4. klienti: çmimi dhe porosia ------------------------------------------------------
const checkout = {
  merchant_id: merchant.id,
  items: [{ product_id: product.id, option_ids: [], quantity: 2 }],
  payment_method: 'cash',
  fulfillment: 'courier',
  address_line1: 'Sunny Hill 5',
  address: homeAt,
  instructions: 'Kati i dytë',
};
const quote = step('klienti: çmimi', await call('/api/v1/orders/quote', { method: 'POST', token: owner, body: checkout }), (r) => r.status === 200 && r.json.open_now === true);
console.log(`     artikujt ${(quote.items_total_minor / 100).toFixed(2)} + dërgesa ${(quote.delivery_fee_minor / 100).toFixed(2)} = ${(quote.total_minor / 100).toFixed(2)} ${quote.currency}`);
const order = step('klienti: porosia', await call('/api/v1/orders', { method: 'POST', token: owner, body: checkout, idem: true }), (r) => (r.status === 201 || r.status === 200) && r.json.id);
console.log(`     porosia ${order.id} · ${order.state} · kodi ${order.code}`);

// ---- 5. tregtari: pranon → përgatit → gati -------------------------------------------------
const transition = async (to, extra = {}) =>
  step(`tregtari: ${to}`, await call(`/api/v1/merchant/orders/${order.id}/transition`, { method: 'POST', token: owner, body: { to, ...extra } }), (r) => r.status === 200 && r.json.state === to);
await transition('accepted', { prep_time_min: 10 });
await transition('preparing');
await transition('ready');

// ---- 6. kurieri: oferta → pranimi → marrja → dorëzimi ------------------------------------------
let offer = null;
for (let i = 0; i < 15 && !offer; i++) {
  await wait(2000);
  const o = await call('/api/v1/courier/offers', { token: courier });
  offer = (o.json.items ?? []).find((x) => x.order_id === order.id) ?? null;
}
step('kurieri: oferta', { status: offer ? 200 : 404, json: offer ?? { info: 'asnjë ofertë brenda 30 s' } }, () => !!offer);
const assigned = step('kurieri: pranimi', await call(`/api/v1/courier/offers/${offer.id}/accept`, { method: 'POST', token: courier, idem: true }), (r) => r.status === 200);
console.log(`     gjendja: ${assigned.state}`);
step('kurieri: kod i gabuar refuzohet', await call(`/api/v1/courier/orders/${order.id}/pickup`, { method: 'POST', token: courier, body: { code: order.code.replace(/./g, 'Z') } }), (r) => r.status >= 400);
const picked = step('kurieri: marrja me kodin', await call(`/api/v1/courier/orders/${order.id}/pickup`, { method: 'POST', token: courier, body: { code: order.code } }), (r) => r.status === 200);
console.log(`     gjendja: ${picked.state}`);
const delivered = step('kurieri: dorëzimi', await call(`/api/v1/courier/orders/${order.id}/deliver`, { method: 'POST', token: courier, idem: true }), (r) => r.status === 200);
console.log(`     gjendja: ${delivered.state} · pagesa ${delivered.payment_status}`);

// ---- 7. klienti e sheh të dorëzuar ------------------------------------------------------------
const final = step('klienti: porosia e dorëzuar', await call(`/api/v1/orders/${order.id}`, { token: owner }), (r) => r.status === 200 && r.json.state === 'delivered');
console.log(`     totali ${(final.total_minor / 100).toFixed(2)}`);

step('kurieri: jashtë pune', await call('/api/v1/driver/offline', { method: 'POST', token: courier }), (r) => r.status < 300);
console.log('U KRYE — porosi ushqimi nga menyja te dorëzimi');
