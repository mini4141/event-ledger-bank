// Headless screenshots for proof. Usage: node capture/capture.mjs <outDir> <local|eks>
import { chromium } from "@playwright/test";

const [outDir, mode = "local"] = process.argv.slice(2);
const base = process.env.BASE_URL ?? "http://localhost:18080";
const jaeger = process.env.JAEGER_URL ?? "http://localhost:26686";
const names = mode === "eks"
  ? { customers: null, account: "02-eks-account", feed: "01-eks-feed", system: "03-eks-system" }
  : { customers: "01-customers", account: "02-account-entries", transfer: "03-transfer", feed: "04-live-feed", system: "05-system-health" };

const run = Date.now().toString().slice(-4);
const AL = `Alice-${run}`, BO = `Bob-${run}`;
const browser = await chromium.launch();
const ctx = await browser.newContext({ viewport: { width: 1200, height: 800 } });
const page = await ctx.newPage();
const feed = await ctx.newPage();
const shot = (p, name) => p.screenshot({ path: `${outDir}/${name}.png` });

await feed.goto(base);
await feed.getByTestId("nav-feed").click();
await feed.getByText("connected").waitFor();

await page.goto(base);
await page.getByTestId("customer-name").fill(AL);
await page.getByTestId("create-customer").click();
await page.getByTestId(`balance-${AL}`).waitFor();
await page.getByTestId("customer-name").fill(BO);
await page.getByTestId("create-customer").click();
await page.getByTestId(`balance-${BO}`).waitFor();
if (names.customers) await shot(page, names.customers);

// fund the newest Alice, transfer 25.00 to the newest Bob
const alice = page.locator(`[data-testid="row-${AL}"]`).last();
await alice.getByRole("button", { name: "Deposit 100.00" }).click();
await page.getByTestId(`balance-${AL}`).filter({ hasText: "100.00" }).waitFor();
await page.getByTestId("nav-transfer").click();
const val = async (id, name) => page.locator(`[data-testid=${id}] option`, { hasText: name }).getAttribute("value");
await page.getByTestId("from").selectOption(await val("from", AL));
await page.getByTestId("to").selectOption(await val("to", BO));
await page.getByTestId("amount").fill("25.00");
await page.getByTestId("submit-transfer").click();
await page.getByTestId("transfer-result").waitFor();
if (names.transfer) await shot(page, names.transfer);

await feed.getByTestId("feed-item").first().waitFor({ timeout: 5000 });
await shot(feed, names.feed);

await page.getByTestId("nav-customers").click();
await page.locator(`[data-testid="row-${AL}"]`).getByRole("button", { name: "Entries" }).click();
await page.getByTestId("entries").locator("tbody tr").first().waitFor();
await shot(page, names.account);

await page.getByTestId("nav-system").click();
await page.getByText("UP").first().waitFor();
await page.waitForTimeout(500);
await shot(page, names.system);

if (mode === "local") {
  const res = await fetch(`${jaeger}/api/traces?service=transfers&limit=1`).then((r) => r.json());
  const id = res.data?.[0]?.traceID;
  if (id) {
    await page.goto(`${jaeger}/trace/${id}`);
    await page.waitForTimeout(2500);
    await shot(page, "06-jaeger-transfer-trace");
  }
}
await browser.close();
