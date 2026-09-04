'use client';

import { useRef, useState } from 'react';

import { useSession } from '@/components/session-provider';
import { Badge, Button, Card, Field, Toggle } from '@/components/ui';
import { api, MediaKind, removeMedia, uploadMedia } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { money } from '@/lib/format';
import { Merchant } from '@/lib/types';

import styles from './settings.module.css';

/// Logoja dhe kopertina: një imazh, dy veprime. Skedari shkon drejt në magazinë me URL të
/// nënshkruar; serveri e lidh me vendin dhe klientët e shohin brenda pak sekondash.
function MediaItem({
  label,
  kind,
  url,
  merchantId,
  logo = false,
  onDone,
}: {
  label: string;
  kind: MediaKind;
  url: string | null;
  merchantId: string;
  logo?: boolean;
  onDone: (failure: string | null) => Promise<void> | void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);

  async function run(action: () => Promise<unknown>) {
    setBusy(true);
    try {
      await action();
      await onDone(null);
    } catch (e) {
      await onDone(errorText(e));
    } finally {
      setBusy(false);
      if (input.current) input.current.value = '';
    }
  }

  const preview = `${styles.mediaPreview} ${logo ? styles.mediaLogo : ''}`;
  return (
    <div className={styles.mediaItem}>
      <span className={styles.mediaLabel}>{label}</span>
      {url ? (
        // Imazhi vjen nga CloudFront me URL të qëndrueshme; next/image s'ka çfarë të optimizojë këtu.
        // eslint-disable-next-line @next/next/no-img-element
        <img className={preview} src={url} alt={label} />
      ) : (
        <div className={`${preview} ${styles.mediaEmpty}`}>Pa imazh</div>
      )}
      <input
        ref={input}
        className={styles.fileInput}
        type="file"
        accept="image/jpeg,image/png,image/webp"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) void run(() => uploadMedia(kind, merchantId, file));
        }}
      />
      <div className={styles.actions}>
        <Button busy={busy} onClick={() => input.current?.click()}>
          {url ? 'Ndrysho' : 'Ngarko'}
        </Button>
        {url && (
          <Button variant="ghost" busy={busy} onClick={() => run(() => removeMedia(kind, merchantId))}>
            Hiq
          </Button>
        )}
      </div>
    </div>
  );
}

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
          <h2>Brendimi</h2>
          <p className={styles.help}>
            Logoja shfaqet te lista e vendeve, kopertina në krye të menusë. JPEG, PNG ose WebP,
            deri 5 MB; kopertina del më mirë në raport 3:2.
          </p>
          <div className={styles.media}>
            <MediaItem
              label="Logo"
              kind="merchant_logo"
              url={merchant.logo_url ?? null}
              merchantId={merchant.id}
              logo
              onDone={async (f) => {
                setFailure(f);
                if (!f) await reload();
              }}
            />
            <MediaItem
              label="Kopertina"
              kind="merchant_cover"
              url={merchant.cover_url ?? null}
              merchantId={merchant.id}
              onDone={async (f) => {
                setFailure(f);
                if (!f) await reload();
              }}
            />
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
