import { useCallback, useEffect, useState } from "react";
import { api, money, type Account, type Customer, type Entry, type FeedEvent, type Transfer } from "./api";

type Page = "customers" | "account" | "transfer" | "feed" | "system";
const short = (id: string) => id.slice(0, 8);

interface Row { customer: Customer; accounts: Account[] }

async function loadRows(): Promise<Row[]> {
  const customers = await api.customers();
  return Promise.all(customers.map(async (customer) => ({ customer, accounts: await api.accountsOf(customer.customer_id) })));
}

function Customers({ rows, reload, open }: { rows: Row[]; reload: () => void; open: (id: string) => void }) {
  const [name, setName] = useState("");
  const [err, setErr] = useState("");
  const create = async () => {
    try {
      const c = await api.createCustomer(name);
      await api.openAccount(c.customer_id, "GBP");
      setName("");
      reload();
    } catch (e) { setErr(String(e)); }
  };
  const deposit = async (id: string) => { await api.deposit(id, 10000); reload(); };
  return (
    <>
      <section>
        <h2>New customer</h2>
        <div className="row">
          <input data-testid="customer-name" placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} />
          <button className="primary" data-testid="create-customer" disabled={!name} onClick={create}>Create + open GBP account</button>
          <span className="bad">{err}</span>
        </div>
      </section>
      <section>
        <h2>Customers</h2>
        <table>
          <thead><tr><th>Name</th><th>Account</th><th>Balance</th><th></th></tr></thead>
          <tbody>
            {rows.flatMap(({ customer, accounts }) =>
              accounts.map((a) => (
                <tr key={a.account_id} data-testid={`row-${customer.name}`}>
                  <td>{customer.name}</td>
                  <td className="mono">{a.account_id}</td>
                  <td data-testid={`balance-${customer.name}`}>{a.currency} {money(a.balance_minor)}</td>
                  <td className="row">
                    <button className="small" data-testid={`deposit-${customer.name}`} onClick={() => deposit(a.account_id)}>Deposit 100.00</button>
                    <button className="small" onClick={() => open(a.account_id)}>Entries</button>
                  </td>
                </tr>
              )),
            )}
          </tbody>
        </table>
      </section>
    </>
  );
}

function AccountDetail({ id }: { id: string }) {
  const [acct, setAcct] = useState<Account>();
  const [entries, setEntries] = useState<Entry[]>([]);
  useEffect(() => {
    if (!id) return;
    api.account(id).then(setAcct);
    api.entries(id).then(setEntries);
  }, [id]);
  if (!id) return <section>Pick an account on the Customers page.</section>;
  return (
    <section>
      <h2>Account <span className="mono">{id}</span></h2>
      <p data-testid="account-balance">Balance: <b>{acct ? `${acct.currency} ${money(acct.balance_minor)}` : "…"}</b> (version {acct?.version})</p>
      <table data-testid="entries">
        <thead><tr><th>Posted</th><th>Direction</th><th>Amount</th><th>Counterparty</th><th>Posting</th></tr></thead>
        <tbody>
          {entries.map((e) => (
            <tr key={e.posted_at + e.direction}>
              <td className="mono">{e.posted_at}</td>
              <td className={e.direction === "DEBIT" ? "debit" : "credit"}>{e.direction}</td>
              <td>{money(e.amount_minor)}</td>
              <td className="mono">{short(e.counterparty)}</td>
              <td className="mono">{short(e.posting_id)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function TransferForm({ rows, onDone }: { rows: Row[]; onDone: () => void }) {
  const opts = rows.flatMap(({ customer, accounts }) => accounts.map((a) => ({ id: a.account_id, label: `${customer.name} (${short(a.account_id)})` })));
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [amount, setAmount] = useState("25.00");
  const [key, setKey] = useState(() => crypto.randomUUID());
  const [res, setRes] = useState<Transfer>();
  const [err, setErr] = useState("");
  const submit = async () => {
    setErr("");
    try {
      setRes(await api.transfer(key, from, to, Math.round(Number(amount) * 100)));
      onDone();
    } catch (e) { setErr(String(e)); }
  };
  return (
    <section>
      <h2>Transfer</h2>
      <div className="row">
        <select data-testid="from" value={from} onChange={(e) => setFrom(e.target.value)}>
          <option value="">From…</option>
          {opts.map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}
        </select>
        <select data-testid="to" value={to} onChange={(e) => setTo(e.target.value)}>
          <option value="">To…</option>
          {opts.map((o) => <option key={o.id} value={o.id}>{o.label}</option>)}
        </select>
        <input data-testid="amount" value={amount} onChange={(e) => setAmount(e.target.value)} size={8} />
        <button className="primary" data-testid="submit-transfer" disabled={!from || !to} onClick={submit}>Send</button>
        <button className="small" onClick={() => setKey(crypto.randomUUID())}>New idempotency key</button>
      </div>
      <p className="mono">idempotency key: {key} (submit twice with the same key to see the replay)</p>
      {res && <p data-testid="transfer-result">Transfer <span className="mono">{res.transfer_id}</span> — {res.status}{res.replayed ? " (replayed, no double spend)" : ""}</p>}
      <p className="bad">{err}</p>
    </section>
  );
}

function Feed() {
  const [events, setEvents] = useState<FeedEvent[]>([]);
  const [live, setLive] = useState(false);
  useEffect(() => {
    const es = new EventSource("/v1/feed/stream");
    es.onopen = () => setLive(true);
    es.onerror = () => setLive(false);
    es.onmessage = (m) => setEvents((prev) => [JSON.parse(m.data) as FeedEvent, ...prev].slice(0, 100));
    return () => es.close();
  }, []);
  return (
    <section>
      <h2>Live event feed <span className={live ? "ok" : "bad"}>{live ? "● connected" : "○ connecting"}</span></h2>
      <ul className="feed" data-testid="feed">
        {events.map((e) => (
          <li key={e.transfer_id + e.type} data-testid="feed-item">
            <b>{e.type}</b> · {money(e.amount_minor)} · <span className="mono">{short(e.from_account_id)} → {short(e.to_account_id)}</span>
            {e.reason && <span className="bad"> · {e.reason}</span>} · <span className="mono">{e.occurred_at}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

const SERVICES = ["accounts", "ledger", "transfers", "notifications", "analytics-sink"];

function System() {
  const [health, setHealth] = useState<Record<string, boolean>>({});
  const [clusters, setClusters] = useState<{ name: string; healthy: number; total: number }[]>([]);
  useEffect(() => {
    SERVICES.forEach((s) => api.health(s).then((ok) => setHealth((h) => ({ ...h, [s]: ok })), () => setHealth((h) => ({ ...h, [s]: false }))));
    api.clusters().then((r) =>
      setClusters((r.cluster_statuses ?? []).map((c) => ({
        name: c.name,
        total: c.host_statuses?.length ?? 0,
        healthy: (c.host_statuses ?? []).filter((h) => h.health_status?.eds_health_status === "HEALTHY").length,
      }))),
    );
  }, []);
  return (
    <>
      <section>
        <h2>Service health (via Envoy /healthz routes)</h2>
        <table data-testid="health">
          <tbody>{SERVICES.map((s) => <tr key={s}><td>{s}</td><td className={health[s] ? "ok" : "bad"}>{health[s] === undefined ? "…" : health[s] ? "UP" : "DOWN"}</td></tr>)}</tbody>
        </table>
      </section>
      <section>
        <h2>Envoy clusters</h2>
        <table>
          <thead><tr><th>Cluster</th><th>Healthy hosts</th></tr></thead>
          <tbody>{clusters.map((c) => <tr key={c.name}><td>{c.name}</td><td>{c.healthy} / {c.total}</td></tr>)}</tbody>
        </table>
      </section>
    </>
  );
}

export default function App() {
  const [page, setPage] = useState<Page>("customers");
  const [acct, setAcct] = useState("");
  const [rows, setRows] = useState<Row[]>([]);
  const reload = useCallback(() => { loadRows().then(setRows); }, []);
  useEffect(reload, [reload]);
  const nav = (p: Page) => (
    <button className={page === p ? "active" : ""} onClick={() => { setPage(p); reload(); }} data-testid={`nav-${p}`}>
      {p[0].toUpperCase() + p.slice(1)}
    </button>
  );
  return (
    <>
      <header>
        <h1>Event Ledger Bank</h1>
        <nav>{nav("customers")}{nav("account")}{nav("transfer")}{nav("feed")}{nav("system")}</nav>
      </header>
      <main>
        {page === "customers" && <Customers rows={rows} reload={reload} open={(id) => { setAcct(id); setPage("account"); }} />}
        {page === "account" && <AccountDetail id={acct} />}
        {page === "transfer" && <TransferForm rows={rows} onDone={reload} />}
        {page === "feed" && <Feed />}
        {page === "system" && <System />}
      </main>
    </>
  );
}
