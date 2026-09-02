/// Format e të dhënave që kthen API-ja, të kufizuara në atë që përdor paneli.
/// Burimi i vërtetë mbetet openapi.yaml; këtu rrinë vetëm fushat që shfaqen.

export type Me = {
  id: string;
  phone: string | null;
  email: string | null;
  full_name: string | null;
  locale: string;
  capabilities: string[];
};

export type AdminRide = {
  id: string;
  state: string;
  category: string;
  customer_id: string;
  driver_id: string | null;
  payment_method: string;
  payment_status: string;
  price_quoted_minor: number;
  price_final_minor: number | null;
  pickup_address: string | null;
  dropoff_address: string | null;
  requested_at: string;
  completed_at: string | null;
  cancelled_by: string | null;
};

export type DispatchLive = {
  rides: AdminRide[];
  counts: Record<string, number>;
  online_drivers: Record<string, number>;
  open_offers: number;
  safety_open: number;
  generated_at: string;
};

export type AdminUser = {
  id: string;
  phone: string | null;
  email: string | null;
  full_name: string | null;
  locale: string;
  status: string;
  capabilities: string[];
  created_at: string;
};

export type DriverProfile = {
  user_id: string;
  status: string;
  vehicle_make: string;
  vehicle_model: string;
  vehicle_plate: string;
  vehicle_color: string;
  categories: string[];
  rating: number | null;
  rating_count: number;
  suspended_reason: string | null;
  created_at: string;
};

export type DriverDocument = {
  id: string;
  driver_id?: string;
  type: string;
  status: string;
  content_type: string;
  size_bytes: number;
  expires_on: string | null;
  rejection_reason: string | null;
  reviewed_at: string | null;
  created_at: string;
  download_url?: string;
};

export type DocumentsOverview = {
  documents: DriverDocument[];
  missing: string[];
  expiring: string[];
  eligible: boolean;
};

export type Ticket = {
  id: string;
  user_id: string;
  category: string;
  subject: string;
  status: string;
  priority: string;
  ride_id: string | null;
  last_message_at: string;
  created_at: string;
  messages?: TicketMessage[];
};

export type TicketMessage = {
  id: string;
  author_role: string;
  body: string;
  created_at: string;
};

export type Flag = {
  key: string;
  enabled: boolean;
  rollout_percent: number;
  public: boolean;
  description: string | null;
};

export type RiskFlag = {
  id: string;
  user_id: string;
  kind: string;
  severity: string;
  score: number;
  status: string;
  note: string | null;
  created_at: string;
  resolved_at: string | null;
};

export type Items<T> = { items: T[] };

/// Kapacitetet që i japin kuptim menysë. Serveri i zbaton gjithsesi; kjo vetëm fsheh
/// atë që përdoruesi nuk e bën dot, që paneli të mos ofrojë butona që kthejnë 403 (§37, §52).
export const CAP = {
  admin: 'ADMIN',
  operations: 'OPERATIONS',
  support: 'SUPPORT',
  finance: 'FINANCE',
} as const;

export function can(me: Me | null, ...caps: string[]): boolean {
  if (!me) return false;
  if (me.capabilities.includes(CAP.admin)) return true;
  return caps.some((c) => me.capabilities.includes(c));
}

/// Stafi njihet nga këto kapacitete; një klient i zakonshëm nuk hyn dot në panel.
export function isStaff(me: Me | null): boolean {
  return can(me, CAP.operations, CAP.support, CAP.finance);
}
