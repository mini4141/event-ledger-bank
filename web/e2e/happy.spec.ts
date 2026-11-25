import { expect, test } from "@playwright/test";

test("create two customers, deposit, transfer, live feed shows the event", async ({ page, context }) => {
  const run = Date.now().toString().slice(-6);
  const alice = `Alice-${run}`;
  const bob = `Bob-${run}`;

  const feed = await context.newPage();
  await feed.goto("/");
  await feed.getByTestId("nav-feed").click();
  await expect(feed.getByText("connected")).toBeVisible();

  await page.goto("/");
  for (const n of [alice, bob]) {
    await page.getByTestId("customer-name").fill(n);
    await page.getByTestId("create-customer").click();
    await expect(page.getByTestId(`balance-${n}`)).toContainText("0.00");
  }
  await page.getByTestId(`deposit-${alice}`).click();
  await expect(page.getByTestId(`balance-${alice}`)).toContainText("100.00");

  await page.getByTestId("nav-transfer").click();
  const optionValue = (testid: string, name: string) =>
    page.locator(`[data-testid=${testid}] option`, { hasText: name }).getAttribute("value");
  await page.getByTestId("from").selectOption((await optionValue("from", alice))!);
  await page.getByTestId("to").selectOption((await optionValue("to", bob))!);
  await page.getByTestId("amount").fill("25.00");
  await page.getByTestId("submit-transfer").click();
  await expect(page.getByTestId("transfer-result")).toContainText("COMPLETED");
  await page.screenshot({ path: "test-results/transfer.png" });

  await expect(feed.getByTestId("feed-item").first()).toContainText("25.00", { timeout: 2000 });

  await page.getByTestId("nav-customers").click();
  await expect(page.getByTestId(`balance-${alice}`)).toContainText("75.00");
  await expect(page.getByTestId(`balance-${bob}`)).toContainText("25.00");
});
