'use client';

import { useCallback, useState } from 'react';

import { useSession } from '@/components/session-provider';
import { Badge, Button, Card, Empty, ErrorState, Loading } from '@/components/ui';
import { api } from '@/lib/api';
import { errorText } from '@/lib/errors';
import { money } from '@/lib/format';
import { Menu, Product } from '@/lib/types';
import { usePoll } from '@/lib/use-poll';

import styles from './menu.module.css';

/// Menuja e stafit. Veprimi që bëhet dhjetëra herë në ditë është të fikësh diçka që mbaroi,
/// ndaj ai është një prekje e vetme; ndryshimet e tjera bëhen rrallë dhe jetojnë te Cilësimet.
export default function MenuPage() {
  const { merchant } = useSession();
  const [busy, setBusy] = useState<string | null>(null);
  const [failure, setFailure] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!merchant) return null;
    return api.get<Menu>(`merchant/${merchant.id}/menu`);
  }, [merchant]);

  const { data, error, loading, refresh } = usePoll(load);

  async function setAvailable(product: Product, available: boolean) {
    if (!merchant) return;
    setBusy(product.id);
    setFailure(null);
    try {
      await api.patch(`merchant/${merchant.id}/products/${product.id}/availability`, { available });
      refresh();
    } catch (e) {
      setFailure(errorText(e));
    } finally {
      setBusy(null);
    }
  }

  if (!merchant) return null;

  // Një produkt pa kategori, ose me një kategori që s'është më aktive, do të humbiste
  // nga lista po të grupohej vetëm sipas kategorive të njohura.
  const known = new Set((data?.categories ?? []).map((c) => c.id));
  const uncategorised = (data?.products ?? []).filter(
    (p) => p.category_id === null || !known.has(p.category_id),
  );

  return (
    <>
      <header className={styles.head}>
        <h1>Menuja</h1>
        <span className={styles.hint}>Prek një artikull për ta fikur ose ndezur</span>
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
      ) : data.products.length === 0 ? (
        <Empty
          title="Menuja është bosh"
          message="Artikujt shtohen nga paneli i KREJT-it kur vendi regjistrohet."
        />
      ) : (
        <>
          {data.categories.map((c) => {
            const products = data.products.filter((p) => p.category_id === c.id);
            if (products.length === 0) return null;
            return (
              <Section key={c.id} title={c.name}>
                {products.map((p) => (
                  <ProductRow
                    key={p.id}
                    product={p}
                    busy={busy === p.id}
                    onToggle={() => setAvailable(p, !p.available)}
                  />
                ))}
              </Section>
            );
          })}

          {uncategorised.length > 0 && (
            <Section title="Pa kategori">
              {uncategorised.map((p) => (
                <ProductRow
                  key={p.id}
                  product={p}
                  busy={busy === p.id}
                  onToggle={() => setAvailable(p, !p.available)}
                />
              ))}
            </Section>
          )}
        </>
      )}
    </>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className={styles.section}>
      <h2 className={styles.sectionTitle}>{title}</h2>
      <div className={styles.grid}>{children}</div>
    </section>
  );
}

function ProductRow({
  product,
  busy,
  onToggle,
}: {
  product: Product;
  busy: boolean;
  onToggle: () => void;
}) {
  return (
    <Card tone={product.available ? 'ok' : 'muted'}>
      <div className={styles.row}>
        <div className={styles.info}>
          <strong className={product.available ? styles.name : styles.nameOff}>
            {product.name}
          </strong>
          <span className={`${styles.price} num`}>
            {money(product.price_minor, product.currency)}
          </span>
        </div>
        {!product.available && <Badge tone="muted">Fikur</Badge>}
      </div>
      <Button
        size="lg"
        variant={product.available ? 'danger' : 'primary'}
        busy={busy}
        onClick={onToggle}
      >
        {product.available ? 'Fike' : 'Ndize'}
      </Button>
    </Card>
  );
}
