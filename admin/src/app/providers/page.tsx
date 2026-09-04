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
import { dateTime } from "@/lib/format";
import { Items, ServiceProvider } from "@/lib/types";
import { usePoll } from "@/lib/use-poll";

const CATEGORY_LABEL: Record<string, string> = {
  electrician: "Elektricist",
  plumber: "Hidraulik",
  cleaning: "Pastrim",
  ac: "Klima",
  appliance: "Pajisje",
  moving: "Bartje",
  handyman: "I përgjithshëm",
};

const STATUS_TONE: Record<string, "ok" | "info" | "danger" | "muted"> = {
  approved: "ok",
  pending: "info",
  suspended: "danger",
};

/// Mjeshtrit e shërbimeve. Miratimi hap punën për ta menjëherë, ndaj shqyrtimi bëhet me sy:
/// kategoritë që kërkon, qyteti dhe kontakti publik.
export default function ProvidersPage() {
  const load = useCallback(
    () =>
      api
        .get<Items<ServiceProvider>>("admin/service-providers")
        .then((r) => r.items ?? []),
    [],
  );
  const { data, error, loading, refresh } = usePoll(load);

  const [busy, setBusy] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);
  const [reason, setReason] = useState("");

  async function setStatus(
    p: ServiceProvider,
    status: "approved" | "suspended" | "pending",
  ) {
    setBusy(p.user_id);
    setFailure(null);
    try {
      await api.patch(`admin/service-providers/${p.user_id}`, {
        status,
        reason: status === "suspended" ? reason : "",
      });
      setReason("");
      refresh();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <>
      <PageHeader title="Mjeshtrit" />

      {failure && (
        <p style={{ color: "var(--danger)", fontSize: 13 }} role="alert">
          {failure}
        </p>
      )}

      {loading && !data ? (
        <Loading />
      ) : !data ? (
        <ErrorState message={errorText(error)} onRetry={refresh} />
      ) : data.length === 0 ? (
        <Card>
          <Empty
            title="Asnjë mjeshtër"
            message="Aplikimet nga aplikacioni i shoferit shfaqen këtu për shqyrtim."
          />
        </Card>
      ) : (
        <>
          <Card title="Arsyeja e pezullimit">
            <Field
              label="Shkruhet te llogaria e mjeshtrit"
              value={reason}
              onChange={setReason}
              placeholder="p.sh. dokumente të pavlefshme"
              maxLength={200}
            />
          </Card>
          <Card>
            <div className="scroll-x">
              <table>
                <thead>
                  <tr>
                    <th>Emri</th>
                    <th>Qyteti</th>
                    <th>Kategoritë</th>
                    <th>Punë</th>
                    <th>Vlerësimi</th>
                    <th>Aplikoi</th>
                    <th>Gjendja</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {data.map((p) => (
                    <tr key={p.user_id}>
                      <td>{p.business_name ?? "—"}</td>
                      <td>{p.city}</td>
                      <td style={{ color: "var(--muted)" }}>
                        {p.categories
                          .map((c) => CATEGORY_LABEL[c] ?? c)
                          .join(", ")}
                      </td>
                      <td className="num">{p.jobs_done}</td>
                      <td className="num">
                        {p.rating === null
                          ? "—"
                          : `${p.rating} (${p.rating_count})`}
                      </td>
                      <td style={{ color: "var(--muted)" }}>
                        {dateTime(p.created_at)}
                      </td>
                      <td>
                        <Badge tone={STATUS_TONE[p.status] ?? "muted"}>
                          {p.status}
                        </Badge>
                      </td>
                      <td>
                        <div style={{ display: "flex", gap: 8 }}>
                          {p.status !== "approved" && (
                            <Button
                              busy={busy === p.user_id}
                              onClick={() => setStatus(p, "approved")}
                            >
                              Mirato
                            </Button>
                          )}
                          {p.status !== "suspended" && (
                            <Button
                              variant="danger"
                              busy={busy === p.user_id}
                              onClick={() => setStatus(p, "suspended")}
                            >
                              Pezullo
                            </Button>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </Card>
        </>
      )}
    </>
  );
}
