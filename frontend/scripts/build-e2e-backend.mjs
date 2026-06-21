import { mkdirSync } from "node:fs"
import { spawnSync } from "node:child_process"
import path from "node:path"
import { fileURLToPath } from "node:url"

const frontendDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..")
const rootDir = path.resolve(frontendDir, "..")
const tmpDir = path.join(rootDir, "tmp")
const binary = path.join(tmpDir, "playwright-node.exe")
const goCache = path.join(tmpDir, "go-build")

mkdirSync(tmpDir, { recursive: true })
mkdirSync(goCache, { recursive: true })
const result = spawnSync("go", ["build", "-o", binary, "./cmd/node"], {
  cwd: rootDir,
  env: { ...process.env, GOCACHE: goCache },
  stdio: "inherit",
  windowsHide: true,
})
if (result.status !== 0) process.exit(result.status ?? 1)
