import assert from "node:assert/strict";
import { test } from "node:test";

import { CAP, can, isStaff, type Me } from "./types.ts";

const me = (capabilities: string[]): Me => ({
  id: "u1",
  phone: "+38344123456",
  email: null,
  full_name: "Arta Krasniqi",
  locale: "sq",
  capabilities,
});

test("ADMIN i mbulon të gjitha kapacitetet", () => {
  const admin = me([CAP.admin]);
  assert.ok(can(admin, CAP.operations));
  assert.ok(can(admin, CAP.support));
  assert.ok(can(admin, CAP.finance));
});

test("secili kapacitet hap vetëm atë që i takon", () => {
  const support = me([CAP.support]);
  assert.ok(can(support, CAP.support));
  assert.ok(!can(support, CAP.finance));
  assert.ok(!can(support, CAP.operations));
});

test("mjafton njëri nga kapacitetet e kërkuara", () => {
  const ops = me([CAP.operations]);
  assert.ok(can(ops, CAP.operations, CAP.support));
});

test("pa sesion nuk lejohet asgjë", () => {
  assert.ok(!can(null, CAP.operations));
  assert.ok(!isStaff(null));
});

test("një klient i zakonshëm nuk është staf", () => {
  assert.ok(!isStaff(me([])));
  assert.ok(!isStaff(me(["RIDE_DRIVER", "MERCHANT"])));
  assert.ok(isStaff(me([CAP.support])));
  assert.ok(isStaff(me([CAP.admin])));
});

test("SUPER_ADMIN është staf dhe i mbulon të gjitha kapacitetet", () => {
  // Administratori i parë lind vetëm me këtë; pa të, paneli e kthente mbrapsht pas kyçjes.
  const root = me([CAP.superAdmin]);
  assert.ok(isStaff(root));
  assert.ok(can(root, CAP.operations));
  assert.ok(can(root, CAP.finance));
});
