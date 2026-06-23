import AxeBuilder from "@axe-core/playwright"
import { expect, test, type Page } from "@playwright/test"

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => localStorage.removeItem("mnemokv.apiBaseUrl"))
})

test("navigates primary learning and operational routes", async ({ page }) => {
  await page.goto("/")
  await expect(page.getByRole("heading", { level: 1, name: "MnemoKV" })).toBeVisible()

  await page.getByRole("link", { name: "Learn", exact: true }).click()
  await expect(page).toHaveURL(/\/learn$/)
  await expect(page.getByRole("heading", { level: 1, name: "Learn" })).toBeVisible()

  await page.getByRole("link", { name: "Use", exact: true }).click()
  await expect(page).toHaveURL(/\/use$/)
  await expect(page.getByRole("heading", { level: 1, name: "Configure a node" })).toBeVisible()

  for (const [tab, path, heading] of [
    ["Dashboard", "/use/dashboard", "Dashboard"],
    ["Console", "/use/console", "Command console"],
    ["Workloads", "/use/workloads", "Workloads"],
    ["Cluster", "/use/cluster", "Cluster"],
    ["Eviction Lab", "/use/eviction", "Eviction lab"],
    ["Benchmarks", "/use/benchmarks", "Benchmarks"],
  ] as const) {
    await page.getByRole("link", { name: tab, exact: true }).click()
    await expect(page).toHaveURL(new RegExp(`${path}$`))
    await expect(page.getByRole("heading", { level: 1, name: heading })).toBeVisible()
  }
})

test("switches API targets and presents an offline state", async ({ page }) => {
  await page.goto("/use/dashboard")
  await expect(page.getByText("Connected to")).toBeVisible()

  const target = page.getByLabel("API base URL")
  await target.fill("http://127.0.0.1:7399")
  await page.getByRole("button", { name: "Connect" }).click()
  await expect(page.getByText(/No node is responding/)).toBeVisible()
  await expect(page.getByText(/Backend not reachable/)).toBeVisible()
  await expect.poll(() => page.evaluate(() => localStorage.getItem("mnemokv.apiBaseUrl"))).toBe(
    "http://127.0.0.1:7399",
  )

  await target.fill("http://127.0.0.1:7380")
  await page.getByRole("button", { name: "Connect" }).click()
  await expect(page.getByText("Connected to")).toBeVisible()
})

test("shows real dashboard data and executes commands", async ({ page }) => {
  await page.goto("/use/dashboard")
  await expect(page.locator("main").getByText("node-1", { exact: true }).last()).toBeVisible()
  await expect(page.locator("main").getByText("standalone", { exact: true }).last()).toBeVisible()

  await page.getByRole("link", { name: "Console", exact: true }).click()
  const command = page.getByLabel("RESP command")
  await command.fill("SET browser:smoke verified")
  await page.getByRole("button", { name: "Send command" }).click()
  await expect(page.getByRole("log")).toContainText("OK")

  await command.fill("GET browser:smoke")
  await page.getByRole("button", { name: "Send command" }).click()
  await expect(page.getByRole("log")).toContainText('"verified"')
})

test("reports malformed API data instead of treating it as ordinary offline state", async ({ page }) => {
  await page.route("**/health", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ status: 42 }) }),
  )
  await page.goto("/use/dashboard")
  await expect(page.getByRole("alert")).toContainText("unexpected response")
  await expect(page.getByRole("alert")).toContainText("status must be a string")
})

test("loads the built-in benchmark example", async ({ page }) => {
  await page.goto("/use/benchmarks")
  await page.getByRole("button", { name: "Load built-in example" }).click()
  await expect(page.getByText(/loaded built-in example/)).toBeVisible()
  await expect(page.getByRole("img", { name: /All benchmarks/ })).toBeVisible()
})

test("shows automatic recovery progress and degraded slot warnings", async ({ page }) => {
  await page.route("**/cluster/state", (route) => route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      enabled: true,
      nodeId: "node-2",
      clusterId: "demo-auto-cluster",
      slotCount: 2,
      metadataVersion: 12,
      routingMode: "proxy",
      failoverMode: "automatic",
      peers: ["node-1", "node-2", "node-3", "node-4", "node-5"],
      membership: [],
      slots: [
        { number: 0, leaderId: "node-2", replicaId: "node-3", localRole: "leader", term: 2, lastSequence: 4, lastAppliedSequence: 0, replicaReady: false },
        { number: 1, leaderId: "node-3", replicaId: "node-4", localRole: "none", term: 1, lastSequence: 2, lastAppliedSequence: 0, replicaReady: true },
      ],
      recovery: {
        state: "repairing",
        controlIndex: 44,
        failedNodes: ["node-1"],
        oneCopySlots: [{
          slot: 0,
          classification: "replica_lost",
          formerLeaderId: "node-2",
          formerReplicaId: "node-1",
          failures: ["node-1"],
          readsAvailable: true,
          writesAvailable: false,
          rejectedCommands: ["SET"],
          message: "slot has one reachable authoritative copy",
        }],
        activePlan: { id: "plan-1", kind: "recovery", reason: "node-1 failed", completedSteps: 2, totalSteps: 5 },
        warning: "another failure before repair completes may cause slot unavailability or data loss",
      },
    }),
  }))
  await page.goto("/use/cluster")
  await expect(page.getByRole("status")).toContainText("repairing")
  await expect(page.getByRole("status")).toContainText("2/5 steps")
  await expect(page.getByRole("status")).toContainText("another failure")
  await expect(page.getByText("1 one-copy / 0 unavailable")).toBeVisible()
  await expect(page.getByText(/replica_lost/)).toBeVisible()
})

for (const viewport of [
  { name: "laptop", width: 1366, height: 768 },
  { name: "projector", width: 1920, height: 1080 },
  { name: "tablet", width: 768, height: 1024 },
]) {
  test(`${viewport.name} layout has no horizontal overflow`, async ({ page }) => {
    await page.setViewportSize(viewport)
    for (const path of ["/", "/learn", "/use", "/use/dashboard", "/use/cluster", "/use/benchmarks"]) {
      await page.goto(path)
      await expect(page.locator("main")).toBeVisible()
      expect(await horizontalOverflow(page), `${path} overflow at ${viewport.name}`).toBeLessThanOrEqual(1)
    }
  })
}

test("keyboard focus, reduced motion, charts, and WCAG checks pass", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" })
  await page.goto("/")
  await page.keyboard.press("Tab")
  await expect(page.getByRole("link", { name: "Skip to main content" })).toBeFocused()
  await page.keyboard.press("Enter")
  await expect(page.locator("main")).toBeFocused()
  await assertAccessible(page)

  for (const path of ["/learn", "/use", "/use/dashboard", "/use/console", "/use/benchmarks"]) {
    await page.goto(path)
    await assertAccessible(page)
  }
})

async function horizontalOverflow(page: Page): Promise<number> {
  return page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
}

async function assertAccessible(page: Page) {
  const results = await new AxeBuilder({ page }).include("#root").withTags(["wcag2a", "wcag2aa"]).analyze()
  expect(results.violations, results.violations.map((item) => `${item.id}: ${item.help}`).join("\n")).toEqual([])
}
