'use client';

import { useCallback, useState } from 'react';

import { Drawer, PageHeader, Row } from '@/components/page';
import { Badge, Button, Card, Empty, ErrorState, Field, Loading, Tone } from '@/components/ui';
import { api } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { dateTime, shortId } from '@/lib/format';
import { DocumentsOverview, DriverProfile, Items } from '@/lib/types';
import { usePoll } from '@/lib/use-poll';

const DOC_LABELS: Record<string, string> = {
  driving_license: 'Patenta',
  id_card: 'Letërnjoftimi',
  vehicle_registration: 'Libreza e automjetit',
  insurance: 'Siguracioni',
  criminal_record: 'Dëshmia e penalitetit',
  profile_photo: 'Fotografia e profilit',
  taxi_permit: 'Licenca e taksisë',
};

const DOC_STATUS: Record<string, { label: string; tone: Tone }> = {
  pending: { label: 'Në shqyrtim', tone: 'info' },
  approved: { label: 'Aprovuar', tone: 'ok' },
  rejected: { label: 'Refuzuar', tone: 'danger' },
  expired: { label: 'Skaduar', tone: 'danger' },
  replaced: { label: 'Zëvendësuar', tone: 'muted' },
};

const DRIVER_STATUS: Record<string, { label: string; tone: Tone }> = {
  pending: { label: 'Në pritje', tone: 'warn' },
  approved: { label: 'Aprovuar', tone: 'ok' },
  suspended: { label: 'Pezulluar', tone: 'danger' },
};

/// Verifikimi i shoferëve. Miratimi kërkon dokumentet e aprovuara; serveri e refuzon nëse mungojnë,
/// ndaj butoni këtu tregon të njëjtën gjë para se ta provosh (§31).
export default function DriversPage() {
  const [open, setOpen] = useState<DriverProfile | null>(null);
  const load = useCallback(
    () => api.get<Items<DriverProfile>>('admin/drivers').then((r) => r.items ?? []),
    [],
  );
  const { data, error, loading, refresh } = usePoll(load);

  return (
    <>
      <PageHeader title="Shoferët në pritje" />

      {loading && !data ? (
        <Loading />
      ) : !data ? (
        <ErrorState message={errorText(error)} onRetry={refresh} />
      ) : data.length === 0 ? (
        <Card>
          <Empty
            title="Asnjë aplikim në pritje"
            message="Kur një shofer i ri të dërgojë dokumentet, shfaqet këtu."
          />
        </Card>
      ) : (
        <Card>
          <div className="scroll-x">
            <table>
              <thead>
                <tr>
                  <th>Statusi</th>
                  <th>Automjeti</th>
                  <th>Targa</th>
                  <th>Kategoritë</th>
                  <th>Aplikuar</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {data.map((d) => {
                  const s = DRIVER_STATUS[d.status] ?? { label: d.status, tone: 'muted' as Tone };
                  return (
                    <tr key={d.user_id}>
                      <td>
                        <Badge tone={s.tone}>{s.label}</Badge>
                      </td>
                      <td>
                        {d.vehicle_color} {d.vehicle_make} {d.vehicle_model}
                      </td>
                      <td className="num">{d.vehicle_plate}</td>
                      <td>{d.categories.join(', ') || '—'}</td>
                      <td className="num">{dateTime(d.created_at)}</td>
                      <td>
                        <Button variant="ghost" onClick={() => setOpen(d)}>
                          Shqyrto
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
        <DriverReview
          driver={open}
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

function DriverReview({
  driver,
  onClose,
  onChanged,
}: {
  driver: DriverProfile;
  onClose: () => void;
  onChanged: () => void;
}) {
  const load = useCallback(
    () => api.get<DocumentsOverview>(`admin/drivers/${driver.user_id}/documents`),
    [driver.user_id],
  );
  const { data, error, loading, refresh } = usePoll(load);

  const [busy, setBusy] = useState(false);
  const [reason, setReason] = useState('');
  const [failure, setFailure] = useState<string | null>(null);

  async function decideDocument(id: string, action: 'approve' | 'reject') {
    if (action === 'reject' && reason.trim().length === 0) {
      setFailure('Refuzimi kërkon një arsye që shoferi ta dijë çfarë të ndreqë.');
      return;
    }
    setBusy(true);
    setFailure(null);
    try {
      await api.patch(`admin/driver-documents/${id}`, {
        action,
        ...(action === 'reject' ? { reason: reason.trim() } : {}),
      });
      setReason('');
      refresh();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(false);
    }
  }

  async function decideDriver(action: 'approve' | 'suspend') {
    if (action === 'suspend' && reason.trim().length === 0) {
      setFailure('Pezullimi kërkon një arsye; ajo ruhet te audit log-u.');
      return;
    }
    setBusy(true);
    setFailure(null);
    try {
      await api.patch(`admin/drivers/${driver.user_id}`, {
        action,
        ...(action === 'approve' ? { categories: driver.categories } : { reason: reason.trim() }),
      });
      onChanged();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Drawer title={`Shoferi ${shortId(driver.user_id)}`} onClose={onClose}>
      <Row label="Automjeti" value={`${driver.vehicle_color} ${driver.vehicle_make} ${driver.vehicle_model}`} />
      <Row label="Targa" value={driver.vehicle_plate} />
      <Row label="Kategoritë" value={driver.categories.join(', ') || '—'} />
      <Row label="Aplikuar" value={dateTime(driver.created_at)} />
      {driver.suspended_reason && <Row label="Arsyeja e pezullimit" value={driver.suspended_reason} />}

      <h2 style={{ marginTop: 8 }}>Dokumentet</h2>

      {loading && !data ? (
        <Loading />
      ) : !data ? (
        <ErrorState message={errorText(error)} onRetry={refresh} />
      ) : (
        <>
          {data.missing.length > 0 && (
            <p style={{ color: 'var(--warn)', fontSize: 13, margin: 0 }}>
              Mungojnë: {data.missing.map((m) => DOC_LABELS[m] ?? m).join(', ')}
            </p>
          )}

          {data.documents.map((doc) => {
            const s = DOC_STATUS[doc.status] ?? { label: doc.status, tone: 'muted' as Tone };
            return (
              <div key={doc.id} style={{ borderBottom: '1px solid var(--line)', paddingBottom: 12 }}>
                <Row
                  label={DOC_LABELS[doc.type] ?? doc.type}
                  value={<Badge tone={s.tone}>{s.label}</Badge>}
                />
                {doc.expires_on && <Row label="Skadon" value={doc.expires_on} />}
                {doc.rejection_reason && <Row label="Arsyeja" value={doc.rejection_reason} />}
                <div style={{ display: 'flex', gap: 8, marginTop: 8, flexWrap: 'wrap' }}>
                  {doc.download_url && (
                    // Lidhja e nënshkruar skadon për pesë minuta; hapet veç, nuk ruhet askund.
                    <a href={doc.download_url} target="_blank" rel="noopener noreferrer">
                      Hap dokumentin
                    </a>
                  )}
                  {doc.status === 'pending' && (
                    <>
                      <Button busy={busy} onClick={() => decideDocument(doc.id, 'approve')}>
                        Aprovo
                      </Button>
                      <Button
                        variant="danger"
                        busy={busy}
                        onClick={() => decideDocument(doc.id, 'reject')}
                      >
                        Refuzo
                      </Button>
                    </>
                  )}
                </div>
              </div>
            );
          })}

          <Field
            label="Arsyeja (për refuzim ose pezullim)"
            value={reason}
            onChange={setReason}
            placeholder="Fotoja është e palexueshme"
            error={failure}
          />

          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Button
              busy={busy}
              disabled={!data.eligible}
              onClick={() => decideDriver('approve')}
            >
              Mirato shoferin
            </Button>
            <Button variant="danger" busy={busy} onClick={() => decideDriver('suspend')}>
              Pezullo
            </Button>
          </div>
          {!data.eligible && (
            <p style={{ color: 'var(--muted)', fontSize: 12, margin: 0 }}>
              Miratimi hapet kur të gjitha dokumentet e detyrueshme të jenë aprovuar.
            </p>
          )}
        </>
      )}
    </Drawer>
  );
}
