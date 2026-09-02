'use client';

import { useCallback, useState } from 'react';

import { Chips, Drawer, PageHeader, Row } from '@/components/page';
import { Badge, Button, Card, Empty, ErrorState, Field, Loading, Tone } from '@/components/ui';
import { api } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { ago, dateTime, shortId } from '@/lib/format';
import { Items, Ticket } from '@/lib/types';
import { usePoll } from '@/lib/use-poll';

type StatusFilter = 'open' | 'pending_user' | 'resolved' | 'all';

const FILTERS: { value: StatusFilter; label: string }[] = [
  { value: 'open', label: 'Të hapura' },
  { value: 'pending_user', label: 'Presin përdoruesin' },
  { value: 'resolved', label: 'Zgjidhur' },
  { value: 'all', label: 'Të gjitha' },
];

const STATUS: Record<string, { label: string; tone: Tone }> = {
  open: { label: 'Hapur', tone: 'warn' },
  pending_user: { label: 'Pret përdoruesin', tone: 'info' },
  resolved: { label: 'Zgjidhur', tone: 'ok' },
  closed: { label: 'Mbyllur', tone: 'muted' },
};

const PRIORITY: Record<string, { label: string; tone: Tone }> = {
  urgent: { label: 'Urgjente', tone: 'danger' },
  high: { label: 'E lartë', tone: 'warn' },
  normal: { label: 'Normale', tone: 'muted' },
};

/// Radha e mbështetjes. Urgjentet rrinë lart sepse aty hyjnë raportimet e sigurisë (§36).
export default function SupportPage() {
  const [filter, setFilter] = useState<StatusFilter>('open');
  const [openId, setOpenId] = useState<string | null>(null);

  const load = useCallback(
    () =>
      api
        .get<Items<Ticket>>('admin/support/tickets', { status: filter, limit: 50 })
        .then((r) =>
          [...(r.items ?? [])].sort((a, b) => {
            const rank = (t: Ticket) => (t.priority === 'urgent' ? 0 : t.priority === 'high' ? 1 : 2);
            const d = rank(a) - rank(b);
            return d !== 0 ? d : b.last_message_at.localeCompare(a.last_message_at);
          }),
        ),
    [filter],
  );

  const { data, error, loading, refresh } = usePoll(load, 15000);

  return (
    <>
      <PageHeader title="Mbështetja">
        <Chips options={FILTERS} value={filter} onChange={setFilter} />
      </PageHeader>

      {loading && !data ? (
        <Loading />
      ) : !data ? (
        <ErrorState message={errorText(error)} onRetry={refresh} />
      ) : data.length === 0 ? (
        <Card>
          <Empty title="Radha është bosh" message="Asnjë tiketë me këtë filtër." />
        </Card>
      ) : (
        <Card>
          <div className="scroll-x">
            <table>
              <thead>
                <tr>
                  <th>Prioriteti</th>
                  <th>Statusi</th>
                  <th>Tema</th>
                  <th>Kategoria</th>
                  <th>Mesazhi i fundit</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {data.map((t) => {
                  const s = STATUS[t.status] ?? { label: t.status, tone: 'muted' as Tone };
                  const p = PRIORITY[t.priority] ?? { label: t.priority, tone: 'muted' as Tone };
                  return (
                    <tr key={t.id}>
                      <td>
                        <Badge tone={p.tone}>{p.label}</Badge>
                      </td>
                      <td>
                        <Badge tone={s.tone}>{s.label}</Badge>
                      </td>
                      <td>{t.subject}</td>
                      <td>{t.category}</td>
                      <td className="num">{ago(t.last_message_at)}</td>
                      <td>
                        <Button variant="ghost" onClick={() => setOpenId(t.id)}>
                          Hap
                        </Button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {openId && (
        <TicketDrawer
          id={openId}
          onClose={() => setOpenId(null)}
          onChanged={refresh}
        />
      )}
    </>
  );
}

function TicketDrawer({
  id,
  onClose,
  onChanged,
}: {
  id: string;
  onClose: () => void;
  onChanged: () => void;
}) {
  const load = useCallback(() => api.get<Ticket>(`admin/support/tickets/${id}`), [id]);
  const { data, error, loading, refresh } = usePoll(load);

  const [reply, setReply] = useState('');
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);

  async function send() {
    if (reply.trim().length === 0) return;
    setBusy(true);
    setFailure(null);
    try {
      await api.post(`admin/support/tickets/${id}/messages`, { body: reply.trim() });
      setReply('');
      refresh();
      onChanged();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(false);
    }
  }

  async function setStatus(status: string) {
    setBusy(true);
    setFailure(null);
    try {
      await api.patch(`admin/support/tickets/${id}`, { status });
      refresh();
      onChanged();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Drawer title={data?.subject ?? 'Tiketa'} onClose={onClose}>
      {loading && !data ? (
        <Loading />
      ) : !data ? (
        <ErrorState message={errorText(error)} onRetry={refresh} />
      ) : (
        <>
          <Row label="Kategoria" value={data.category} />
          <Row label="Statusi" value={STATUS[data.status]?.label ?? data.status} />
          <Row label="Prioriteti" value={PRIORITY[data.priority]?.label ?? data.priority} />
          <Row label="Përdoruesi" value={shortId(data.user_id)} />
          {data.ride_id && <Row label="Udhëtimi" value={shortId(data.ride_id)} />}
          <Row label="Hapur" value={dateTime(data.created_at)} />

          <h2 style={{ marginTop: 8 }}>Bisedë</h2>
          {(data.messages ?? []).length === 0 ? (
            <Empty title="Ende pa mesazhe" />
          ) : (
            (data.messages ?? []).map((m) => (
              <div
                key={m.id}
                style={{
                  background: m.author_role === 'user' ? 'var(--surface-2)' : 'var(--brand-100)',
                  borderRadius: 'var(--r-sm)',
                  padding: '10px 12px',
                }}
              >
                <div style={{ fontSize: 11, color: 'var(--muted)', marginBottom: 4 }}>
                  {m.author_role === 'user' ? 'Përdoruesi' : m.author_role === 'system' ? 'Sistemi' : 'Mbështetja'}
                  {' · '}
                  {dateTime(m.created_at)}
                </div>
                <div style={{ fontSize: 14, lineHeight: 1.45 }}>{m.body}</div>
              </div>
            ))
          )}

          <Field
            label="Përgjigju"
            value={reply}
            onChange={setReply}
            placeholder="Shkruaj përgjigjen…"
            error={failure}
          />
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Button busy={busy} disabled={reply.trim().length === 0} onClick={send}>
              Dërgo
            </Button>
            {data.status !== 'resolved' && (
              <Button variant="ghost" busy={busy} onClick={() => setStatus('resolved')}>
                Shëno si të zgjidhur
              </Button>
            )}
          </div>
        </>
      )}
    </Drawer>
  );
}
