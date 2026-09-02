'use client';

import { useCallback, useState } from 'react';

import { Chips, Drawer, PageHeader, Row } from '@/components/page';
import { Badge, Card, Empty, ErrorState, Loading } from '@/components/ui';
import { api } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { dateTime, money, rideState, rideTone, shortId } from '@/lib/format';
import { AdminRide, Items } from '@/lib/types';
import { usePoll } from '@/lib/use-poll';

type StateFilter = 'active' | 'completed' | 'cancelled' | 'no_driver' | 'all';

const FILTERS: { value: StateFilter; label: string }[] = [
  { value: 'active', label: 'Në rrjedhë' },
  { value: 'completed', label: 'Përfunduar' },
  { value: 'cancelled', label: 'Anuluar' },
  { value: 'no_driver', label: 'Pa shofer' },
  { value: 'all', label: 'Të gjitha' },
];

/// Udhëtimet me filtra. "Në rrjedhë" nuk është një gjendje e vetme te serveri, ndaj kërkohet
/// pa filtër dhe ndahet këtu; gjendjet e tjera i lexon serveri drejtpërdrejt.
export default function RidesPage() {
  const [filter, setFilter] = useState<StateFilter>('active');
  const [open, setOpen] = useState<AdminRide | null>(null);

  const load = useCallback(async () => {
    const serverState =
      filter === 'active' || filter === 'all' ? undefined : filter;
    const res = await api.get<Items<AdminRide>>('admin/rides', {
      state: serverState,
      limit: 50,
    });
    const items = res.items ?? [];
    if (filter !== 'active') return items;
    const live = new Set(['matching', 'assigned', 'arrived', 'in_progress']);
    return items.filter((r) => live.has(r.state));
  }, [filter]);

  const { data, error, loading, refresh } = usePoll(load);

  return (
    <>
      <PageHeader title="Udhëtimet">
        <Chips options={FILTERS} value={filter} onChange={setFilter} />
      </PageHeader>

      {loading && !data ? (
        <Loading />
      ) : !data ? (
        <ErrorState message={errorText(error)} onRetry={refresh} />
      ) : data.length === 0 ? (
        <Card>
          <Empty title="Asnjë udhëtim me këtë filtër" />
        </Card>
      ) : (
        <Card>
          <div className="scroll-x">
            <table>
              <thead>
                <tr>
                  <th>Gjendja</th>
                  <th>Kërkuar</th>
                  <th>Nga</th>
                  <th>Te</th>
                  <th>Pagesa</th>
                  <th>Çmimi</th>
                  <th>Shoferi</th>
                </tr>
              </thead>
              <tbody>
                {data.map((r) => (
                  <tr key={r.id} onClick={() => setOpen(r)} style={{ cursor: 'pointer' }}>
                    <td>
                      <Badge tone={rideTone(r.state) as never}>{rideState(r.state)}</Badge>
                    </td>
                    <td className="num">{dateTime(r.requested_at)}</td>
                    <td>{r.pickup_address ?? '—'}</td>
                    <td>{r.dropoff_address ?? '—'}</td>
                    <td>{r.payment_method === 'wallet' ? 'Wallet' : 'Para në dorë'}</td>
                    <td className="num">{money(r.price_final_minor ?? r.price_quoted_minor)}</td>
                    <td className="num">{r.driver_id ? shortId(r.driver_id) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {open && (
        <Drawer title={`Udhëtimi ${shortId(open.id)}`} onClose={() => setOpen(null)}>
          <Row
            label="Gjendja"
            value={<Badge tone={rideTone(open.state) as never}>{rideState(open.state)}</Badge>}
          />
          <Row label="Kategoria" value={open.category} />
          <Row label="Nisja" value={open.pickup_address ?? '—'} />
          <Row label="Destinacioni" value={open.dropoff_address ?? '—'} />
          <Row label="Kërkuar" value={dateTime(open.requested_at)} />
          <Row label="Përfunduar" value={dateTime(open.completed_at)} />
          <Row
            label="Pagesa"
            value={`${open.payment_method === 'wallet' ? 'Wallet' : 'Para në dorë'} · ${open.payment_status}`}
          />
          <Row label="Çmimi i ofertës" value={money(open.price_quoted_minor)} />
          <Row
            label="Çmimi final"
            value={open.price_final_minor === null ? '—' : money(open.price_final_minor)}
          />
          <Row label="Klienti" value={shortId(open.customer_id)} />
          <Row label="Shoferi" value={open.driver_id ? shortId(open.driver_id) : '—'} />
          {open.cancelled_by && <Row label="Anuluar nga" value={open.cancelled_by} />}
        </Drawer>
      )}
    </>
  );
}
