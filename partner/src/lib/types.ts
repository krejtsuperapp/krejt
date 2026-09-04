/// Format që përdor paneli i partnerit. Burimi i vërtetë mbetet openapi.yaml.

export type Me = {
  id: string;
  phone: string | null;
  full_name: string | null;
  locale: string;
  capabilities: string[];
};

export type Merchant = {
  id: string;
  type: string;
  name: string;
  slug: string;
  address_line1: string;
  city: string;
  status: string;
  fulfillment_mode: string;
  min_order_minor: number;
  delivery_fee_minor: number;
  prep_time_min: number;
  accepting_orders: boolean;
  open_now: boolean;
  /// Publike (CloudFront); null pa imazh.
  logo_url: string | null;
  cover_url: string | null;
};

export type OrderItem = {
  id: string;
  name: string;
  options: string[];
  unit_minor: number;
  quantity: number;
  total_minor: number;
};

export type Order = {
  id: string;
  code: string;
  state: string;
  fulfillment: string;
  payment_method: string;
  payment_status: string;
  items_total_minor: number;
  delivery_fee_minor: number;
  total_minor: number;
  currency: string;
  note: string | null;
  prep_time_min: number;
  ready_at_estimate: string | null;
  created_at: string;
  accepted_at: string | null;
  items: OrderItem[];
};

export type ModifierOption = {
  id: string;
  name: string;
  price_delta_minor: number;
  available: boolean;
};

export type ModifierGroup = {
  id: string;
  name: string;
  min_select: number;
  max_select: number;
  options: ModifierOption[];
};

export type Product = {
  id: string;
  category_id: string | null;
  name: string;
  description: string | null;
  price_minor: number;
  currency: string;
  available: boolean;
  unit: string;
  modifiers: ModifierGroup[];
  image_url: string | null;
};

export type MenuCategory = { id: string; name: string; sort: number; active: boolean };

export type Menu = {
  merchant_id: string;
  categories: MenuCategory[];
  products: Product[];
};

export type Items<T> = { items: T[] };

/// Radha e kuzhinës: vetëm gjendjet ku merchant-i ka ende diçka për të bërë.
export const OPEN_STATES = ['pending_merchant', 'accepted', 'preparing', 'ready'];

export function isOpen(order: Order): boolean {
  return OPEN_STATES.includes(order.state);
}

/// Hapi i radhës që i takon merchant-it për një porosi. `null` do të thotë se topi
/// është te korrieri ose te klienti, jo te kuzhina (§19).
export function nextStep(order: Order): { to: string; label: string } | null {
  switch (order.state) {
    case 'pending_merchant':
      return { to: 'accepted', label: 'Prano porosinë' };
    case 'accepted':
      return { to: 'preparing', label: 'Nisi përgatitja' };
    case 'preparing':
      return { to: 'ready', label: 'Gati' };
    case 'ready':
      // Dorëzimin e mbyll merchant-i vetëm kur nuk ka korrier në mes.
      return order.fulfillment === 'courier' ? null : { to: 'delivered', label: 'U dorëzua' };
    default:
      return null;
  }
}

/// Sa urgjente është një porosi: aq sa ka pritur pa u prekur.
export function waitedMinutes(order: Order, now: Date = new Date()): number {
  const since = new Date(order.accepted_at ?? order.created_at);
  if (Number.isNaN(since.getTime())) return 0;
  return Math.max(0, Math.floor((now.getTime() - since.getTime()) / 60000));
}
