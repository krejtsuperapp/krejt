'use client';

import { useCallback, useEffect, useState } from 'react';

import { Drawer, PageHeader, Row, Search } from '@/components/page';
import { Badge, Button, Card, Empty, ErrorState, Field, Loading, Tone } from '@/components/ui';
import { useSession } from '@/components/session-provider';
import { api } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { dateTime, shortId } from '@/lib/format';
import { AdminUser, CAP, can, Items } from '@/lib/types';
import { usePoll } from '@/lib/use-poll';

const STATUS: Record<string, { label: string; tone: Tone }> = {
  active: { label: 'Aktiv', tone: 'ok' },
  blocked: { label: 'Bllokuar', tone: 'danger' },
  deleted: { label: 'Fshirë', tone: 'muted' },
};

/// Kërkimi i përdoruesve. Qasja te detajet auditohet nga serveri, ndaj kërkohet vetëm kur
/// operatori shkruan diçka: një listë e plotë përdoruesish nuk i shërben askujt (§35, §57).
export default function UsersPage() {
  const { me } = useSession();
  const [query, setQuery] = useState('');
  const [debounced, setDebounced] = useState('');
  const [open, setOpen] = useState<AdminUser | null>(null);

  useEffect(() => {
    const t = setTimeout(() => setDebounced(query.trim()), 400);
    return () => clearTimeout(t);
  }, [query]);

  const load = useCallback(async () => {
    if (debounced.length < 3) return [];
    const res = await api.get<Items<AdminUser>>('admin/users', { q: debounced, limit: 30 });
    return res.items ?? [];
  }, [debounced]);

  const { data, error, loading, refresh } = usePoll(load);

  return (
    <>
      <PageHeader title="Përdoruesit">
        <Search
          value={query}
          onChange={setQuery}
          placeholder="Numër telefoni, email ose emër"
        />
      </PageHeader>

      {debounced.length < 3 ? (
        <Card>
          <Empty
            title="Shkruaj të paktën tre shkronja"
            message="Kërkimi hapet vetëm me një term; qasja te një llogari regjistrohet te audit log-u."
          />
        </Card>
      ) : loading && !data ? (
        <Loading />
      ) : !data ? (
        <ErrorState message={errorText(error)} onRetry={refresh} />
      ) : data.length === 0 ? (
        <Card>
          <Empty title="Asnjë përdorues nuk përputhet" />
        </Card>
      ) : (
        <Card>
          <div className="scroll-x">
            <table>
              <thead>
                <tr>
                  <th>Statusi</th>
                  <th>Emri</th>
                  <th>Telefoni</th>
                  <th>Kapacitetet</th>
                  <th>Regjistruar</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {data.map((u) => {
                  const s = STATUS[u.status] ?? { label: u.status, tone: 'muted' as Tone };
                  return (
                    <tr key={u.id}>
                      <td>
                        <Badge tone={s.tone}>{s.label}</Badge>
                      </td>
                      <td>{u.full_name ?? '—'}</td>
                      <td className="num">{u.phone ?? '—'}</td>
                      <td>{u.capabilities.join(', ') || '—'}</td>
                      <td className="num">{dateTime(u.created_at)}</td>
                      <td>
                        <Button variant="ghost" onClick={() => setOpen(u)}>
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

      {open && (
        <UserDrawer
          user={open}
          canBlock={can(me, CAP.operations)}
          onClose={() => setOpen(null)}
          onChanged={() => {
            setOpen(null);
            refresh();
          }}
        />
      )}
    </>
  );
}

function UserDrawer({
  user,
  canBlock,
  onClose,
  onChanged,
}: {
  user: AdminUser;
  canBlock: boolean;
  onClose: () => void;
  onChanged: () => void;
}) {
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const blocked = user.status === 'blocked';

  async function block() {
    if (reason.trim().length === 0) {
      setFailure('Bllokimi kërkon arsye: ajo ruhet te audit log-u dhe i thuhet përdoruesit.');
      return;
    }
    setBusy(true);
    setFailure(null);
    try {
      await api.post(`admin/users/${user.id}/block`, { reason: reason.trim() });
      onChanged();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(false);
    }
  }

  async function unblock() {
    setBusy(true);
    setFailure(null);
    try {
      await api.post(`admin/users/${user.id}/unblock`);
      onChanged();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Drawer title={user.full_name ?? shortId(user.id)} onClose={onClose}>
      <Row label="Identifikuesi" value={shortId(user.id)} />
      <Row label="Telefoni" value={user.phone ?? '—'} />
      <Row label="Email" value={user.email ?? '—'} />
      <Row label="Gjuha" value={user.locale} />
      <Row label="Statusi" value={STATUS[user.status]?.label ?? user.status} />
      <Row label="Kapacitetet" value={user.capabilities.join(', ') || '—'} />
      <Row label="Regjistruar" value={dateTime(user.created_at)} />

      {canBlock ? (
        blocked ? (
          <Button busy={busy} onClick={unblock}>
            Zhblloko
          </Button>
        ) : (
          <>
            <Field
              label="Arsyeja e bllokimit"
              value={reason}
              onChange={setReason}
              placeholder="Përpjekje mashtrimi me pagesa"
              error={failure}
            />
            <Button variant="danger" busy={busy} onClick={block}>
              Blloko përdoruesin
            </Button>
            <p style={{ color: 'var(--muted)', fontSize: 12, margin: 0 }}>
              Bllokimi i shkyç menjëherë të gjitha sesionet e kësaj llogarie.
            </p>
          </>
        )
      ) : (
        <p style={{ color: 'var(--muted)', fontSize: 12, margin: 0 }}>
          Bllokimi kërkon kapacitetin OPERATIONS.
        </p>
      )}
    </Drawer>
  );
}
