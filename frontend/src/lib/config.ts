export type Mode = "standalone" | "clustered"
export type EvictionPolicy = "noeviction" | "fifo" | "lru" | "lfu" | "random"
export type WriteSafety = "async" | "strong"

export type Peer = {
  id: string
  address: string
}

export type NodeConfig = {
  id: string
  mode: Mode
  dataDir: string
  bindAddr: string
  port: number
  apiBindAddr: string
  apiPort: number
  stripeCount: number
  memoryLimitBytes: number
  evictionPolicy: EvictionPolicy
  shardingEnabled: boolean
  replicationEnabled: boolean
  autoFailover: boolean
  writeSafetyMode: WriteSafety
  peers: Peer[]
}

export function defaultStandalone(): NodeConfig {
  return {
    id: "node-1",
    mode: "standalone",
    dataDir: "./data",
    bindAddr: "127.0.0.1",
    port: 6380,
    apiBindAddr: "127.0.0.1",
    apiPort: 7380,
    stripeCount: 32,
    memoryLimitBytes: 0,
    evictionPolicy: "lru",
    shardingEnabled: false,
    replicationEnabled: false,
    autoFailover: false,
    writeSafetyMode: "async",
    peers: [],
  }
}

export function defaultClusterPeers(count: number): Peer[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `node-${i + 1}`,
    address: `127.0.0.1:${6381 + i}`,
  }))
}

export function configToYaml(c: NodeConfig): string {
  const lines: string[] = []
  lines.push("node:")
  lines.push(`  id: ${c.id}`)
  lines.push(`  mode: ${c.mode}`)
  lines.push(`  dataDir: ${c.dataDir}`)
  lines.push("")
  lines.push("network:")
  lines.push(`  bindAddr: ${c.bindAddr}`)
  lines.push(`  port: ${c.port}`)
  lines.push("  maxConnections: 1024")
  lines.push("  readTimeoutMs: 30000")
  lines.push("  writeTimeoutMs: 30000")
  lines.push("")
  lines.push("engine:")
  lines.push(`  stripeCount: ${c.stripeCount}`)
  lines.push(`  memoryLimitBytes: ${c.memoryLimitBytes}`)
  lines.push(`  evictionPolicy: ${c.evictionPolicy}`)
  lines.push("")
  lines.push("cluster:")
  lines.push(`  enabled: ${c.mode === "clustered"}`)
  lines.push(`  shardingEnabled: ${c.shardingEnabled}`)
  lines.push(`  replicationEnabled: ${c.replicationEnabled}`)
  lines.push(`  autoFailover: ${c.autoFailover}`)
  lines.push(`  writeSafetyMode: ${c.writeSafetyMode}`)
  if (c.mode === "clustered" && c.peers.length > 0) {
    lines.push("  peers:")
    for (const p of c.peers) {
      lines.push(`    - id: ${p.id}`)
      lines.push(`      address: ${p.address}`)
    }
  } else {
    lines.push("  peers: []")
  }
  lines.push("")
  lines.push("observability:")
  lines.push(`  apiBindAddr: ${c.apiBindAddr}`)
  lines.push(`  apiPort: ${c.apiPort}`)
  lines.push("  logLevel: info")
  lines.push("")
  return lines.join("\n")
}

export type ValidationError = {
  field: string
  message: string
}

export function validate(c: NodeConfig): ValidationError[] {
  const errors: ValidationError[] = []
  if (!c.id.trim()) errors.push({ field: "id", message: "Node id is required" })
  if (c.port < 1 || c.port > 65535) errors.push({ field: "port", message: "Port must be 1-65535" })
  if (c.apiPort < 1 || c.apiPort > 65535) errors.push({ field: "apiPort", message: "API port must be 1-65535" })
  if (c.port === c.apiPort) errors.push({ field: "apiPort", message: "API port must differ from RESP port" })
  if (c.stripeCount < 1) errors.push({ field: "stripeCount", message: "Stripe count must be at least 1" })
  if (c.memoryLimitBytes < 0) errors.push({ field: "memoryLimitBytes", message: "Memory limit cannot be negative" })
  if (c.autoFailover && !c.replicationEnabled) {
    errors.push({ field: "autoFailover", message: "Auto-failover requires replication" })
  }
  if (c.mode === "clustered") {
    if (c.peers.length < 2) {
      errors.push({ field: "peers", message: "A cluster needs at least 2 peers" })
    }
    if (!c.peers.some((p) => p.id === c.id)) {
      errors.push({ field: "peers", message: "This node's id must appear in the peer list" })
    }
  }
  return errors
}
