'use client';

import { useState } from 'react';

import { useSession } from '@/components/session-provider';
import { Badge, Button, Card, Field, Toggle } from '@/components/ui';
import { api } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { money } from '@/lib/format';
import { Merchant } from '@/lib/types';

import styles from './settings.module.css';

/// Cilësimet që ndryshon vetë vendi gjatë ditës. Çmimet dhe tarifat nuk hyjnë këtu:
/// ato i vendos marrëveshja dhe i ndryshon KREJT-i, që klienti të mos gjejë çmim tjetër
/// nga ai që pa te menuja (§19).
export default function SettingsPage() {
  const { merchant, reload } = useSession();
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const [prep, setPrep] = useState<string>(String(merchant?.prep_time_min ?? 0));

  if (!merchant) return null;

  async function patch(body: Record<string, unknown>) {
    setBusy(true);
    setFailure(null);
    try {
      await api.patch<Merchant>(`merchant/${merchant!.id}`, body);
      await reload();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(false);
    }
  }

  const prepMinutes = Number.parseInt(prep, 10);
  const prepValid = Number.isFinite(prepMinutes) && prepMinutes >= 0 && prepMinutes <= 180;

  return (
    <>
      <h1 className={styles.title}>Cilësimet</h1>

      {failure && (
        <p className={styles.failure} role="alert">
          {failure}
        </p>
      )}

      <div className={styles.grid}>
        <Card tone={merchant.accepting_orders ? 'ok' : 'danger'}>
          <h2>Pranimi i porosive</h2>
          <p className={styles.help}>
            Kur e fik, vendi mbetet i dukshëm te klientët por nuk pranon porosi të reja. Porositë
            që janë tashmë në kuzhinë nuk preken.
          </p>
          <Toggle
            label={merchant.accepting_orders ? 'Po pranon porosi' : 'Nuk pranon porosi'}
            checked={merchant.accepting_orders}
            busy={busy}
            onChange={(v) => patch({ accepting_orders: v })}
          />
        </Card>

        <Card>
          <h2>Koha e përgatitjes</h2>
          <p className={styles.help}>
            Kjo kohë i shfaqet klientit kur porosia pranohet. Mbaje afër së vërtetës: një
            premtim i shkurtër që nuk mbahet kushton më shumë se një i gjatë që mbahet.
          </p>
          <Field
            label="Minuta"
            value={prep}
            onChange={(v) => setPrep(v.replace(/\D/g, '').slice(0, 3))}
            inputMode="numeric"
            error={prepValid ? null : 'Vlera duhet nga 0 deri 180 minuta'}
          />
          <div className={styles.actions}>
            <Button
              busy={busy}
              disabled={!prepValid || prepMinutes === merchant.prep_time_min}
              onClick={() => patch({ prep_time_min: prepMinutes })}
            >
              Ruaj
            </Button>
          </div>
        </Card>

        <Card>
          <h2>Vendi</h2>
          <dl className={styles.list}>
            <Row label="Emri" value={merchant.name} />
            <Row label="Adresa" value={`${merchant.address_line1}, ${merchant.city}`} />
            <Row
              label="Gjendja"
              value={
                <Badge tone={merchant.open_now ? 'ok' : 'warn'}>
                  {merchant.open_now ? 'Hapur tani' : 'Jashtë orarit'}
                </Badge>
              }
            />
            <Row label="Porosia minimale" value={money(merchant.min_order_minor)} />
            <Row label="Tarifa e dërgesës" value={money(merchant.delivery_fee_minor)} />
            <Row
              label="Mënyra e dorëzimit"
              value={
                merchant.fulfillment_mode === 'pickup'
                  ? 'Marrje në vend'
                  : merchant.fulfillment_mode === 'merchant_delivers'
                    ? 'Dërgesë nga vendi'
                    : 'Korrier i KREJT-it'
              }
            />
          </dl>
          <p className={styles.help}>
            Orari, tarifat dhe mënyra e dorëzimit ndryshohen nga KREJT-i sipas marrëveshjes.
          </p>
        </Card>
      </div>
    </>
  );
}

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className={styles.row}>
      <dt className={styles.rowLabel}>{label}</dt>
      <dd className={styles.rowValue}>{value}</dd>
    </div>
  );
}
