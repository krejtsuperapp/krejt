'use client';

import { useCallback } from 'react';

import { Badge, Card, Empty, ErrorState, Loading, Stat } from '@/components/ui';
import { api } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { ago, clock, money, rideState, rideTone, shortId } from '@/lib/format';
import { DispatchLive } from '@/lib/types';
import { usePoll } from '@/lib/use-poll';

import styles from './dispatch.module.css';

/// Dispatch live. Rifreskohet vetë çdo pesë sekonda: kanali i gjallë me Centrifugo hyn më vonë
/// dhe do ta zërë vendin e kësaj pyetjeje pa ndryshuar asgjë tjetër në faqe (§42).
const REFRESH_MS = 5000;

export default function DispatchPage() {
  const load = useCallback(() => api.get<DispatchLive>('admin/dispatch/live'), []);
  const { data, error, loading, refresh } = usePoll(load, REFRESH_MS);

  if (loading && !data) return <Loading />;
  if (!data) return <ErrorState message={errorText(error)} onRetry={refresh} />;

  const online = Object.values(data.online_drivers).reduce((n, v) => n + v, 0);
  const matching = data.counts.matching ?? 0;
  const active = data.rides.length;

  return (
    <>
      <header className={styles.head}>
        <h1>Dispatch</h1>
        <span className={styles.stamp}>
          përditësuar {ago(data.generated_at)}
          {error ? ' · lidhja u ndërpre' : ''}
        </span>
      </header>

      <div className={styles.stats}>
        <Stat label="Udhëtime aktive" value={active} />
        <Stat
          label="Pa shofer ende"
          value={matching}
          tone={matching > 0 ? 'warn' : undefined}
        />
        <Stat label="Shoferë online" value={online} tone={online === 0 ? 'danger' : 'ok'} />
        <Stat label="Oferta të hapura" value={data.open_offers} />
        <Stat
          label="Siguria"
          value={data.safety_open}
          tone={data.safety_open > 0 ? 'danger' : undefined}
        />
      </div>

      {data.safety_open > 0 && (
        <div className={styles.alarm} role="alert">
          {data.safety_open} raportim sigurie i pazgjidhur. Këto shikohen para gjithçkaje tjetër.
        </div>
      )}

      <div className={styles.columns}>
        <Card title="Shoferët online sipas kategorisë">
          {online === 0 ? (
            <Empty
              title="Asnjë shofer online"
              message="Pa shoferë online asnjë kërkesë nuk caktohet dot."
            />
          ) : (
            <ul className={styles.list}>
              {Object.entries(data.online_drivers).map(([category, count]) => (
                <li key={category} className={styles.listRow}>
                  <span>{category}</span>
                  <strong className="num">{count}</strong>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card title="Gjendjet">
          {Object.keys(data.counts).length === 0 ? (
            <Empty title="Asnjë udhëtim aktiv" />
          ) : (
            <ul className={styles.list}>
              {Object.entries(data.counts).map(([state, count]) => (
                <li key={state} className={styles.listRow}>
                  <Badge tone={rideTone(state) as never}>{rideState(state)}</Badge>
                  <strong className="num">{count}</strong>
                </li>
              ))}
            </ul>
          )}
        </Card>
      </div>

      <Card title="Udhëtimet aktive">
        {data.rides.length === 0 ? (
          <Empty title="Asnjë udhëtim në rrjedhë" message="Kur të vijë një kërkesë, shfaqet këtu." />
        ) : (
          <div className="scroll-x">
            <table>
              <thead>
                <tr>
                  <th>Gjendja</th>
                  <th>Kërkuar</th>
                  <th>Nga</th>
                  <th>Te</th>
                  <th>Kategoria</th>
                  <th>Pagesa</th>
                  <th>Çmimi</th>
                  <th>Shoferi</th>
                </tr>
              </thead>
              <tbody>
                {data.rides.map((r) => (
                  <tr key={r.id}>
                    <td>
                      <Badge tone={rideTone(r.state) as never}>{rideState(r.state)}</Badge>
                    </td>
                    <td className="num">{clock(r.requested_at)}</td>
                    <td className={styles.address}>{r.pickup_address ?? '—'}</td>
                    <td className={styles.address}>{r.dropoff_address ?? '—'}</td>
                    <td>{r.category}</td>
                    <td>{r.payment_method === 'wallet' ? 'Wallet' : 'Para në dorë'}</td>
                    <td className="num">{money(r.price_final_minor ?? r.price_quoted_minor)}</td>
                    <td className="num">{r.driver_id ? shortId(r.driver_id) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </>
  );
}
