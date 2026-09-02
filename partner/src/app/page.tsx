'use client';

import { useCallback, useEffect, useRef, useState } from 'react';

import { useSession } from '@/components/session-provider';
import { Badge, Button, Card, Empty, ErrorState, Loading, Tone } from '@/components/ui';
import { api } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { clock, money, orderState, orderTone, waited } from '@/lib/format';
import { Items, isOpen, nextStep, Order, waitedMinutes } from '@/lib/types';
import { usePoll } from '@/lib/use-poll';

import styles from './queue.module.css';

/// Radha e kuzhinës. Rifreskohet çdo tetë sekonda dhe bie një sinjal kur vjen një porosi e re,
/// sepse askush nuk rri duke parë ekranin gjatë punës (§19).
const REFRESH_MS = 8000;

export default function QueuePage() {
  const { merchant } = useSession();
  const [busy, setBusy] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!merchant) return [];
    const res = await api.get<Items<Order>>(`merchant/${merchant.id}/orders`, { limit: 50 });
    return (res.items ?? []).filter(isOpen);
  }, [merchant]);

  const { data, error, loading, refresh } = usePoll(load, REFRESH_MS);
  useNewOrderChime(data);

  async function transition(order: Order, to: string, extra: Record<string, unknown> = {}) {
    setBusy(order.id);
    setFailure(null);
    try {
      await api.post(`merchant/orders/${order.id}/transition`, { to, ...extra });
      refresh();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(null);
    }
  }

  async function reject(order: Order) {
    const reason = window.prompt('Pse po refuzohet kjo porosi? Arsyeja i shkon klientit.');
    if (reason === null) return;
    if (reason.trim().length === 0) {
      setFailure('Refuzimi kërkon një arsye.');
      return;
    }
    await transition(order, 'rejected', { reason: reason.trim() });
  }

  if (!merchant) return null;

  const fresh = (data ?? []).filter((o) => o.state === 'pending_merchant');
  const working = (data ?? []).filter((o) => o.state !== 'pending_merchant');

  return (
    <>
      <header className={styles.head}>
        <h1>Porositë</h1>
        <div className={styles.headRight}>
          {!merchant.accepting_orders && <Badge tone="danger">Porositë janë të ndalura</Badge>}
          {!merchant.open_now && <Badge tone="warn">Jashtë orarit</Badge>}
          <span className={styles.count}>{(data ?? []).length} të hapura</span>
        </div>
      </header>

      {failure && (
        <p className={styles.failure} role="alert">
          {failure}
        </p>
      )}

      {loading && !data ? (
        <Loading />
      ) : !data ? (
        <ErrorState message={errorText(error)} onRetry={refresh} />
      ) : data.length === 0 ? (
        <Empty
          title="Asnjë porosi e hapur"
          message="Kur të vijë një porosi e re, shfaqet këtu dhe bie një sinjal."
        />
      ) : (
        <>
          {fresh.length > 0 && (
            <section className={styles.section}>
              <h2 className={styles.sectionTitle}>Të reja</h2>
              <div className={styles.grid}>
                {fresh.map((o) => (
                  <OrderCard
                    key={o.id}
                    order={o}
                    busy={busy === o.id}
                    onAdvance={() => transition(o, 'accepted', { prep_time_min: merchant.prep_time_min })}
                    onReject={() => reject(o)}
                  />
                ))}
              </div>
            </section>
          )}

          {working.length > 0 && (
            <section className={styles.section}>
              <h2 className={styles.sectionTitle}>Në punë</h2>
              <div className={styles.grid}>
                {working.map((o) => {
                  const step = nextStep(o);
                  return (
                    <OrderCard
                      key={o.id}
                      order={o}
                      busy={busy === o.id}
                      onAdvance={step ? () => transition(o, step.to) : undefined}
                    />
                  );
                })}
              </div>
            </section>
          )}
        </>
      )}
    </>
  );
}

function OrderCard({
  order,
  busy,
  onAdvance,
  onReject,
}: {
  order: Order;
  busy: boolean;
  onAdvance?: () => void;
  onReject?: () => void;
}) {
  const step = nextStep(order);
  const minutes = waitedMinutes(order);
  // Pas njëzet minutash pritjeje karta bëhet e kuqe: vonesa duhet parë, jo kërkuar.
  const late = minutes >= 20;

  return (
    <Card tone={(late ? 'danger' : orderTone(order.state)) as Tone}>
      <div className={styles.cardTop}>
        <span className={`${styles.code} num`}>{order.code}</span>
        <Badge tone={orderTone(order.state) as Tone}>{orderState(order.state)}</Badge>
      </div>

      <div className={styles.meta}>
        <span className={late ? styles.lateText : undefined}>{waited(minutes)}</span>
        <span>·</span>
        <span>{clock(order.created_at)}</span>
        <span>·</span>
        <span>{order.fulfillment === 'pickup' ? 'Marrje në vend' : 'Dërgesë'}</span>
        <span>·</span>
        <span>{order.payment_method === 'wallet' ? 'Paguar' : 'Para në dorë'}</span>
      </div>

      <ul className={styles.items}>
        {order.items.map((it) => (
          <li key={it.id} className={styles.item}>
            <span className={`${styles.qty} num`}>{it.quantity}×</span>
            <span className={styles.itemName}>
              {it.name}
              {it.options.length > 0 && (
                <span className={styles.options}>{it.options.join(', ')}</span>
              )}
            </span>
          </li>
        ))}
      </ul>

      {order.note && <p className={styles.note}>{order.note}</p>}

      <div className={styles.total}>
        <span>Totali</span>
        <strong className="num">{money(order.total_minor, order.currency)}</strong>
      </div>

      <div className={styles.actions}>
        {onAdvance && step && (
          <Button size="lg" busy={busy} onClick={onAdvance}>
            {step.label}
          </Button>
        )}
        {!onAdvance && !step && (
          <p className={styles.waitingCourier}>Pret korrierin</p>
        )}
        {onReject && (
          <Button variant="danger" busy={busy} onClick={onReject}>
            Refuzo
          </Button>
        )}
      </div>
    </Card>
  );
}

/// Sinjali i porosisë së re. Krijohet me Web Audio, që të mos varet nga një skedar tingulli,
/// dhe bie vetëm kur numri i porosive të reja rritet — jo në çdo rifreskim.
function useNewOrderChime(orders: Order[] | null) {
  const previous = useRef<number | null>(null);

  useEffect(() => {
    if (!orders) return;
    const fresh = orders.filter((o) => o.state === 'pending_merchant').length;
    const before = previous.current;
    previous.current = fresh;
    if (before === null || fresh <= before) return;

    try {
      const Ctor =
        window.AudioContext ??
        (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
      if (!Ctor) return;
      const ctx = new Ctor();
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = 'sine';
      osc.frequency.value = 880;
      gain.gain.setValueAtTime(0.0001, ctx.currentTime);
      gain.gain.exponentialRampToValueAtTime(0.2, ctx.currentTime + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + 0.6);
      osc.connect(gain).connect(ctx.destination);
      osc.start();
      osc.stop(ctx.currentTime + 0.6);
      osc.onended = () => void ctx.close();
    } catch {
      // Pa leje tingulli paneli punon njësoj; sinjali është ndihmë, jo kusht.
    }
  }, [orders]);
}
