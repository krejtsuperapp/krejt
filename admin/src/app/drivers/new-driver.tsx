"use client";

import { useState } from "react";

import { Drawer, Row } from "@/components/page";
import { Badge, Button, Field } from "@/components/ui";
import { api } from "@/lib/api";
import { errorText } from "@/lib/errors";
import { DriverProfile } from "@/lib/types";

const CATEGORIES = ["economy", "comfort", "xl", "taxi"] as const;

const CATEGORY_LABELS: Record<string, string> = {
  economy: "Economy",
  comfort: "Comfort",
  xl: "XL",
  taxi: "Taksi",
};

/** Regjistrim i një shoferi nga zyra, kur të dhënat e automjetit lexohen nga letrat.
 *  Numri duhet të ketë hyrë një herë në aplikacion: paneli regjistron shoferë, nuk krijon llogari. */
export function NewDriver({
  onClose,
  onCreated,
}: {
  onClose: () => void;
  onCreated: () => void;
}) {
  const [phone, setPhone] = useState("+383");
  const [make, setMake] = useState("");
  const [model, setModel] = useState("");
  const [plate, setPlate] = useState("");
  const [color, setColor] = useState("");
  const [categories, setCategories] = useState<string[]>(["economy"]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const complete =
    /^\+[1-9][0-9]{6,14}$/.test(phone) &&
    make.trim() &&
    model.trim() &&
    plate.trim() &&
    color.trim() &&
    categories.length > 0;

  function toggle(c: string) {
    setCategories((cur) =>
      cur.includes(c) ? cur.filter((x) => x !== c) : [...cur, c],
    );
  }

  async function submit() {
    setBusy(true);
    setError(null);
    try {
      await api.post<DriverProfile>("admin/drivers", {
        phone: phone.trim(),
        vehicle_make: make.trim(),
        vehicle_model: model.trim(),
        vehicle_plate: plate.trim().toUpperCase(),
        vehicle_color: color.trim(),
        categories,
      });
      onCreated();
      onClose();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Drawer title="Regjistro shofer" onClose={onClose}>
      <Field
        label="Numri i telefonit"
        value={phone}
        onChange={setPhone}
        placeholder="+38344123456"
        autoFocus
      />
      <p style={{ color: "var(--muted)", fontSize: 12, margin: "-4px 0 4px" }}>
        Numri duhet të ketë hyrë një herë në aplikacion.
      </p>

      <Field
        label="Marka"
        value={make}
        onChange={setMake}
        placeholder="Volkswagen"
      />
      <Field
        label="Modeli"
        value={model}
        onChange={setModel}
        placeholder="Passat"
      />
      <Field
        label="Targa"
        value={plate}
        onChange={setPlate}
        placeholder="01-123-AB"
      />
      <Field
        label="Ngjyra"
        value={color}
        onChange={setColor}
        placeholder="E zezë"
      />

      <Row
        label="Kategoritë"
        value={
          <span style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
            {CATEGORIES.map((c) => (
              <button
                key={c}
                type="button"
                onClick={() => toggle(c)}
                style={{
                  background: "none",
                  border: 0,
                  padding: 0,
                  cursor: "pointer",
                }}
              >
                <Badge tone={categories.includes(c) ? "ok" : "muted"}>
                  {CATEGORY_LABELS[c]}
                </Badge>
              </button>
            ))}
          </span>
        }
      />

      {error && <p style={{ color: "var(--danger)", fontSize: 13 }}>{error}</p>}

      <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
        <Button onClick={submit} busy={busy} disabled={!complete}>
          Regjistro
        </Button>
        <Button variant="ghost" onClick={onClose}>
          Anulo
        </Button>
      </div>
    </Drawer>
  );
}
