'use client';

import { useCallback, useState } from 'react';

import { Chips, PageHeader } from '@/components/page';
import { Badge, Button, Card, Empty, ErrorState, Field, Loading, Tone } from '@/components/ui';
import { api } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { dateTime, shortId } from '@/lib/format';
import { Items, RiskFlag } from '@/lib/types';
import { usePoll } from '@/lib/use-poll';

type StatusFilter = 'open' | 'reviewing' | 'confirmed' | 'dismissed' | 'all';

const FILTERS: { value: StatusFilter; label: string }[] = [
  { value: 'open', label: 'Të hapura' },
  { value: 'reviewing', label: 'Në shqyrtim' },
  { value: 'confirmed', label: 'Konfirmuar' },
  { value: 'dismissed', label: 'Hedhur poshtë' },
  { value: 'all', label: 'Të gjitha' },
];

const SEVERITY: Record<string, { label: string; tone: Tone }> = {
  high: { label: 'I lartë', tone: 'danger' },
  medium: { label: 'I mesëm', tone: 'warn' },
  low: { label: 'I ulët', tone: 'muted' },
};

/// Flamujt e riskut. Sistemi i ngre; njeriu vendos. Asnjë veprim automatik nuk bllokon llogari (§67).
export default function RiskPage() {
  const [filter, setFilter] = useState<StatusFilter>('open');
  const [note, setNote] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  const load = useCallback(
    () =>
      api
        .get<Items<RiskFlag>>('admin/risk/flags', { status: filter, limit: 50 })
        .then((r) => r.items ?? []),
    [filter],
  );
  const { data, error, loading, refresh } = usePoll(load);

  async function resolve(flag: RiskFlag, status: 'reviewing' | 'confirmed' | 'dismissed') {
    setBusy(flag.id);
    setFailure(null);
    try {
      const text = (note[flag.id] ?? '').trim();
      await api.patch(`admin/risk/flags/${flag.id}`, {
        status,
        ...(text ? { note: text } : {}),
      });
      setNote((n) => ({ ...n, [flag.id]: '' }));
      refresh();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <>
      <PageHeader title="Risku">
        <Chips options={FILTERS} value={filter} onChange={setFilter} />
      </PageHeader>

      {failure && (
        <p style={{ color: 'var(--danger)', fontSize: 13 }} role="alert">
          {failure}
        </p>
      )}

      {loading && !data ? (
        <Loading />
      ) : !data ? (
        <ErrorState message={errorText(error)} onRetry={refresh} />
      ) : data.length === 0 ? (
        <Card>
          <Empty title="Asnjë flamur me këtë filtër" />
        </Card>
      ) : (
        <div style={{ display: 'grid', gap: 12 }}>
          {data.map((f) => {
            const s = SEVERITY[f.severity] ?? { label: f.severity, tone: 'muted' as Tone };
            const settled = f.status === 'confirmed' || f.status === 'dismissed';
            return (
              <Card key={f.id}>
                <div style={{ display: 'flex', gap: 8, alignItems: 'center', flexWrap: 'wrap' }}>
                  <Badge tone={s.tone}>{s.label}</Badge>
                  <strong>{f.kind}</strong>
                  <span className="num" style={{ color: 'var(--muted)', fontSize: 13 }}>
                    pikë {f.score}
                  </span>
                  <span style={{ marginLeft: 'auto', color: 'var(--muted)', fontSize: 13 }}>
                    {dateTime(f.created_at)}
                  </span>
                </div>
                <p style={{ color: 'var(--text-dim)', fontSize: 13, margin: '8px 0' }}>
                  Përdoruesi {shortId(f.user_id)} · statusi {f.status}
                </p>
                {f.note && (
                  <p style={{ color: 'var(--muted)', fontSize: 13, margin: '0 0 8px' }}>{f.note}</p>
                )}
                {!settled && (
                  <>
                    <Field
                      label="Shënim"
                      value={note[f.id] ?? ''}
                      onChange={(v) => setNote((n) => ({ ...n, [f.id]: v }))}
                      placeholder="Çfarë u kontrollua"
                    />
                    <div style={{ display: 'flex', gap: 8, marginTop: 8, flexWrap: 'wrap' }}>
                      <Button
                        variant="ghost"
                        busy={busy === f.id}
                        onClick={() => resolve(f, 'reviewing')}
                      >
                        Merre në shqyrtim
                      </Button>
                      <Button
                        variant="danger"
                        busy={busy === f.id}
                        onClick={() => resolve(f, 'confirmed')}
                      >
                        Konfirmo
                      </Button>
                      <Button
                        variant="ghost"
                        busy={busy === f.id}
                        onClick={() => resolve(f, 'dismissed')}
                      >
                        Hidhe poshtë
                      </Button>
                    </div>
                  </>
                )}
              </Card>
            );
          })}
        </div>
      )}
    </>
  );
}
