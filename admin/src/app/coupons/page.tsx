"use client";

import { useCallback, useState } from "react";

import { PageHeader } from "@/components/page";
import {
  Badge,
  Button,
  Card,
  Empty,
  ErrorState,
  Field,
  Loading,
} from "@/components/ui";
import { api } from "@/lib/api";
import { errorText } from "@/lib/errors";
import { dateTime, money } from "@/lib/format";
import { Coupon, Items } from "@/lib/types";
import { usePoll } from "@/lib/use-poll";

const SCOPES = [
  { value: "all", label: "Të gjitha" },
  { value: "food", label: "Ushqim" },
  { value: "parcels", label: "Pako" },
];

/// Kuponat e zbritjes. Zbritjen e llogarit serveri; këtu vendosen vetëm rregullat, dhe kostoja
/// e tyre shkon te llogaria e marketingut — partneri dhe korrieri marrin të njëjtën shumë (§35).
export default function CouponsPage() {
  const load = useCallback(
    () => api.get<Items<Coupon>>("admin/coupons").then((r) => r.items ?? []),
    [],
  );
  const { data, error, loading, refresh } = usePoll(load);

  const [code, setCode] = useState("");
  const [kind, setKind] = useState<"percent" | "fixed">("percent");
  const [amount, setAmount] = useState("10");
  const [minOrder, setMinOrder] = useState("");
  const [scope, setScope] = useState("all");
  const [maxUses, setMaxUses] = useState("");
  const [perUser, setPerUser] = useState("1");
  const [busy, setBusy] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  async function create() {
    setBusy("create");
    setFailure(null);
    try {
      const value = Number(amount.replace(",", "."));
      await api.post("admin/coupons", {
        code,
        kind,
        percent_bp: kind === "percent" ? Math.round(value * 100) : 0,
        amount_minor: kind === "fixed" ? Math.round(value * 100) : 0,
        min_order_minor: minOrder
          ? Math.round(Number(minOrder.replace(",", ".")) * 100)
          : 0,
        scope,
        max_uses: maxUses ? Number(maxUses) : undefined,
        max_uses_per_user: perUser ? Number(perUser) : undefined,
        active: true,
      });
      setCode("");
      refresh();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(null);
    }
  }

  async function toggle(c: Coupon) {
    setBusy(c.code);
    setFailure(null);
    try {
      await api.patch(`admin/coupons/${c.code}`, { active: !c.active });
      refresh();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(null);
    }
  }

  const valueLabel = (c: Coupon) =>
    c.kind === "percent"
      ? `${(c.percent_bp / 100).toFixed(0)}%`
      : money(c.amount_minor);

  return (
    <>
      <PageHeader title="Kupona" />

      {failure && (
        <p style={{ color: "var(--danger)", fontSize: 13 }} role="alert">
          {failure}
        </p>
      )}

      <Card title="Kupon i ri">
        <div
          style={{
            display: "grid",
            gap: 12,
            gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))",
          }}
        >
          <Field
            label="Kodi"
            value={code}
            onChange={setCode}
            placeholder="KREJT10"
            maxLength={32}
          />
          <label style={{ display: "grid", gap: 6, fontSize: 13 }}>
            <span style={{ color: "var(--muted)" }}>Lloji</span>
            <select
              value={kind}
              onChange={(e) => setKind(e.target.value as "percent" | "fixed")}
              style={{ padding: "10px 12px", borderRadius: 10 }}
            >
              <option value="percent">Përqindje</option>
              <option value="fixed">Shumë fikse</option>
            </select>
          </label>
          <Field
            label={kind === "percent" ? "Përqindja (%)" : "Shuma (€)"}
            value={amount}
            onChange={setAmount}
          />
          <Field
            label="Minimumi (€)"
            value={minOrder}
            onChange={setMinOrder}
            placeholder="0"
          />
          <label style={{ display: "grid", gap: 6, fontSize: 13 }}>
            <span style={{ color: "var(--muted)" }}>Fusha</span>
            <select
              value={scope}
              onChange={(e) => setScope(e.target.value)}
              style={{ padding: "10px 12px", borderRadius: 10 }}
            >
              {SCOPES.map((s) => (
                <option key={s.value} value={s.value}>
                  {s.label}
                </option>
              ))}
            </select>
          </label>
          <Field
            label="Përdorime gjithsej"
            value={maxUses}
            onChange={setMaxUses}
            placeholder="pa kufi"
          />
          <Field
            label="Për përdorues"
            value={perUser}
            onChange={setPerUser}
            placeholder="pa kufi"
          />
        </div>
        <div style={{ marginTop: 12 }}>
          <Button
            busy={busy === "create"}
            onClick={create}
            disabled={code.trim().length < 3}
          >
            Ruaj kuponin
          </Button>
        </div>
      </Card>

      {loading && !data ? (
        <Loading />
      ) : !data ? (
        <ErrorState message={errorText(error)} onRetry={refresh} />
      ) : data.length === 0 ? (
        <Card>
          <Empty
            title="Asnjë kupon"
            message="Kuponat e krijuar këtu vlejnë menjëherë te aplikacioni."
          />
        </Card>
      ) : (
        <Card>
          <div className="scroll-x">
            <table>
              <thead>
                <tr>
                  <th>Kodi</th>
                  <th>Vlera</th>
                  <th>Minimumi</th>
                  <th>Fusha</th>
                  <th>Përdorime</th>
                  <th>Skadon</th>
                  <th>Gjendja</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {data.map((c) => (
                  <tr key={c.code}>
                    <td className="num">{c.code}</td>
                    <td className="num">{valueLabel(c)}</td>
                    <td className="num">
                      {c.min_order_minor ? money(c.min_order_minor) : "—"}
                    </td>
                    <td>
                      {SCOPES.find((s) => s.value === c.scope)?.label ??
                        c.scope}
                    </td>
                    <td className="num">
                      {c.uses_count}
                      {c.max_uses ? ` / ${c.max_uses}` : ""}
                    </td>
                    <td style={{ color: "var(--muted)" }}>
                      {c.ends_at ? dateTime(c.ends_at) : "—"}
                    </td>
                    <td>
                      <Badge tone={c.active ? "ok" : "muted"}>
                        {c.active ? "Aktiv" : "Fikur"}
                      </Badge>
                    </td>
                    <td>
                      <Button
                        variant={c.active ? "danger" : "primary"}
                        busy={busy === c.code}
                        onClick={() => toggle(c)}
                      >
                        {c.active ? "Fike" : "Ndize"}
                      </Button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}
    </>
  );
}
