// All calls go to Envoy's JSON-transcoded REST routes (same origin).
export interface Customer { customer_id: string; name: string; created_at: string }
export interface Account { account_id: string; customer_id: string; currency: string; balance_minor: string; version: number }
export interface Entry { posted_at: string; posting_id: string; direction: string; amount_minor: string; counterparty: string }
export interface FeedEvent { type: string; transfer_id: string; from_account_id: string; to_account_id: string; amount_minor: number; reason?: string; occurred_at: string }
export interface Transfer { transfer_id: string; status: string; replayed: boolean }

async function call<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  if (!res.ok) throw new Error(`${res.status} ${text}`);
  return text ? (JSON.parse(text) as T) : ({} as T);
}

export const money = (minor: string | number) => (Number(minor) / 100).toFixed(2);

export const api = {
  customers: () => call<{ customers?: Customer[] }>("GET", "/v1/customers").then((r) => r.customers ?? []),
  createCustomer: (name: string) => call<Customer>("POST", "/v1/customers", { name }),
  openAccount: (customer_id: string, currency = "GBP") => call<Account>("POST", "/v1/accounts", { customer_id, currency }),
  accountsOf: (customer_id: string) => call<{ accounts?: Account[] }>("GET", `/v1/customers/${customer_id}/accounts`).then((r) => r.accounts ?? []),
  account: (id: string) => call<Account>("GET", `/v1/accounts/${id}`),
  entries: (id: string) => call<{ entries?: Entry[] }>("GET", `/v1/accounts/${id}/entries`).then((r) => r.entries ?? []),
  deposit: (account_id: string, amount_minor: number) =>
    call<Account>("POST", "/v1/deposits", { account_id, amount_minor, idempotency_key: crypto.randomUUID() }),
  transfer: (idempotency_key: string, from_account_id: string, to_account_id: string, amount_minor: number) =>
    call<Transfer>("POST", "/v1/transfers", { idempotency_key, from_account_id, to_account_id, amount_minor }),
  health: (svc: string) => fetch(`/healthz/${svc}`).then((r) => r.ok),
  clusters: () => call<{ cluster_statuses?: { name: string; host_statuses?: { health_status?: { eds_health_status?: string } }[] }[] }>("GET", "/v1/system/health"),
};
