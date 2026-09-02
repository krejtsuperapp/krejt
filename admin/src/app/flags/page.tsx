'use client';

import { useCallback, useState } from 'react';

import { PageHeader } from '@/components/page';
import { Badge, Button, Card, Empty, ErrorState, Loading } from '@/components/ui';
import { api } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { Flag, Items } from '@/lib/types';
import { usePoll } from '@/lib/use-poll';

/// Feature flags. Rollout-i është determinist te serveri: i njëjti përdorues bie gjithmonë
/// në të njëjtën anë, ndaj përqindja nuk e ndryshon përvojën nga një hapje te tjetra (§64).
export default function FlagsPage() {
  const load = useCallback(
    () => api.get<Items<Flag>>('admin/flags').then((r) => r.items ?? []),
    [],
  );
  const { data, error, loading, refresh } = usePoll(load);

  const [busy, setBusy] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  async function update(flag: Flag, patch: Partial<Pick<Flag, 'enabled' | 'rollout_percent' | 'public'>>) {
    setBusy(flag.key);
    setFailure(null);
    try {
      await api.patch(`admin/flags/${flag.key}`, patch);
      refresh();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(null);
    }
  }

  return (
    <>
      <PageHeader title="Feature flags" />

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
          <Empty title="Asnjë flag i regjistruar" />
        </Card>
      ) : (
        <Card>
          <div className="scroll-x">
            <table>
              <thead>
                <tr>
                  <th>Çelësi</th>
                  <th>Gjendja</th>
                  <th>Rollout</th>
                  <th>Publik</th>
                  <th>Përshkrimi</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {data.map((f) => (
                  <tr key={f.key}>
                    <td className="num">{f.key}</td>
                    <td>
                      <Badge tone={f.enabled ? 'ok' : 'muted'}>{f.enabled ? 'Ndezur' : 'Fikur'}</Badge>
                    </td>
                    <td className="num">{f.rollout_percent}%</td>
                    <td>
                      <Badge tone={f.public ? 'info' : 'muted'}>{f.public ? 'Po' : 'Jo'}</Badge>
                    </td>
                    <td style={{ color: 'var(--muted)' }}>{f.description ?? '—'}</td>
                    <td>
                      <Button
                        variant={f.enabled ? 'danger' : 'primary'}
                        busy={busy === f.key}
                        onClick={() => update(f, { enabled: !f.enabled })}
                      >
                        {f.enabled ? 'Fike' : 'Ndize'}
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
