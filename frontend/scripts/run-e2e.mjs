import { spawn, spawnSync } from "node:child_process"
import path from "node:path"
import { fileURLToPath } from "node:url"

const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..")
const rootDir = path.resolve(frontendDir, "..")
const binary = path.join(rootDir, "tmp", "playwright-node.exe")
const config = path.join(rootDir, "configs", "standalone.yaml")
const viteCli = path.join(frontendDir, "node_modules", "vite", "bin", "vite.js")

const backend = spawn(binary, ["-config", config], {
  cwd: rootDir,
  detached: true,
  stdio: "ignore",
  windowsHide: true,
})

if (backend.pid === undefined) throw new Error("Could not start the MnemoKV test node")
backend.unref()

const frontend = spawn(process.execPath, [viteCli, "--host", "127.0.0.1", "--port", "4173"], {
  cwd: frontendDir,
  detached: true,
  stdio: "ignore",
  windowsHide: true,
})

if (frontend.pid === undefined) throw new Error("Could not start the frontend test server")
frontend.unref()

try {
  await Promise.all([
    waitForUrl("http://127.0.0.1:7380/health", "MnemoKV test node"),
    waitForUrl("http://127.0.0.1:4173", "frontend test server"),
  ])
  const playwrightCli = path.join(frontendDir, "node_modules", "@playwright", "test", "cli.js")
  const result = spawnSync(process.execPath, [playwrightCli, "test", ...process.argv.slice(2)], {
    cwd: frontendDir,
    stdio: "inherit",
    windowsHide: true,
  })
  process.exitCode = result.status ?? 1
} finally {
  for (const pid of [frontend.pid, backend.pid]) {
    spawnSync("taskkill", ["/PID", String(pid), "/T", "/F"], {
      stdio: "ignore",
      windowsHide: true,
    })
  }
}

async function waitForUrl(url, name) {
  const deadline = Date.now() + 15_000
  while (Date.now() < deadline) {
    try {
      const response = await fetch(url, {
        headers: { connection: "close" },
      })
      if (response.ok) return
    } catch {
      // The node is still starting.
    }
    await new Promise((resolve) => setTimeout(resolve, 100))
  }
  throw new Error(`${name} did not become ready`)
}
