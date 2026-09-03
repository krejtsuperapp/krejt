// Prova nga fillimi në fund pa dorë: shoferi i provës hyn me kodin fiks (kështu llogaria
// ekziston), administratori i provës e regjistron dhe e aprovon, dhe të dy verifikohen.
// Asnjë sekret nuk shtypet.
const base = process.env.BASE ?? 'https://dev.krejt.app';
const CODE = '111111';
const adminPhone = '+38344100200';
const driverPhone = '+38344100202';

async function call(path, { method = 'GET', body, token } = {}) {
  const res = await fetch(base + path, {
    method,
    headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
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
  console.log(`${good ? 'OK ' : 'DËSHTOI'}  ${name} → ${r.status}${good ? '' : ' ' + JSON.stringify(r.json).slice(0, 220)}`);
  if (!good) process.exit(1);
  return r.json;
};

async function login(phone, who) {
  step(`${who}: kërkesa e kodit`, await call('/api/v1/auth/otp/request', { method: 'POST', body: { phone, locale: 'sq' } }), (r) => r.status === 202 || r.status === 200);
  const pair = step(
    `${who}: kyçja me ${CODE}`,
    await call('/api/v1/auth/otp/verify', {
      method: 'POST',
      body: { phone, code: CODE, locale: 'sq', device: { id: `e2e-${who}`, name: 'Skripti i provës', platform: 'web' } },
    }),
    (r) => r.status === 200 && r.json.access_token,
  );
  const me = step(`${who}: profili`, await call('/api/v1/users/me', { token: pair.access_token }), (r) => r.status === 200);
  console.log(`     ${who}: ${me.id} · të drejtat: ${(me.capabilities ?? []).join(', ')}`);
  return { token: pair.access_token, me };
}

// 1. shoferi hyn një herë — llogaria duhet të ekzistojë para regjistrimit nga zyra
const driver = await login(driverPhone, 'shoferi');
if ((driver.me.capabilities ?? []).includes('SUPER_ADMIN')) {
  console.log('DËSHTOI  shoferi i provës nuk duhet të jetë administrator');
  process.exit(1);
}

// 2. administratori hyn
const admin = await login(adminPhone, 'admin');
if (!(admin.me.capabilities ?? []).includes('SUPER_ADMIN')) {
  console.log('DËSHTOI  administratori i provës nuk ka SUPER_ADMIN');
  process.exit(1);
}

// 3. regjistrimi i shoferit nga zyra (409 = ekziston dhe nuk është 'pending' — vazhdojmë)
const created = step(
  'regjistrimi i shoferit',
  await call('/api/v1/admin/drivers', {
    method: 'POST',
    token: admin.token,
    body: { phone: driverPhone, vehicle_make: 'Volkswagen', vehicle_model: 'Passat', vehicle_plate: '01-123-AB', vehicle_color: 'E zezë', categories: ['economy', 'comfort'] },
  }),
  (r) => r.status === 201 || r.status === 409,
);
const driverID = created.user_id ?? driver.me.id;

// 4. aprovimi (pa dokumente: DOCUMENTS_REQUIRED=false në dev)
const approved = step(
  'aprovimi',
  await call(`/api/v1/admin/drivers/${driverID}`, { method: 'PATCH', token: admin.token, body: { action: 'approve', categories: ['economy', 'comfort'] } }),
  (r) => r.status === 200 || r.status === 409,
);
console.log(`     statusi: ${approved.status ?? '(tashmë i aprovuar)'}`);

// 5. shoferi e sheh vetë profilin e aprovuar
const profile = step('shoferi: profili i tij', await call('/api/v1/driver/profile', { token: driver.token }), (r) => r.status === 200);
console.log(`     ${profile.vehicle_make} ${profile.vehicle_model} · ${profile.vehicle_plate} · ${profile.status}`);

console.log('U KRYE');
