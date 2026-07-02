export type Mode = "standalone" | "clustered"
export type FailoverMode = "manual" | "automatic"
export type EvictionPolicy = "noeviction" | "fifo" | "lru" | "lfu" | "random"

export type Peer = {
  id: string
  address: string
  apiAddress: string
  controlAddress: string
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
  clusterId: string
  shardingEnabled: boolean
  replicationEnabled: boolean
  slotCount: number
  failoverMode: FailoverMode
  controllerControlPort: number
  controllerRaftDir: string
  controllerBootstrapNodeId: string
  controllerObserveIntervalMs: number
  controllerFailureTimeoutMs: number
  controllerConsecutiveFailures: number
  controllerRebalanceSkewThreshold: number
  controllerMigrationRateLimit: number
  controlPlaneRequestSigningSecret: string
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
    clusterId: "demo-cluster",
    shardingEnabled: false,
    replicationEnabled: false,
    slotCount: 1024,
    failoverMode: "manual",
    controllerControlPort: 7481,
    controllerRaftDir: "./data/node-1/controller",
    controllerBootstrapNodeId: "node-1",
    controllerObserveIntervalMs: 1000,
    controllerFailureTimeoutMs: 10000,
    controllerConsecutiveFailures: 3,
    controllerRebalanceSkewThreshold: 1,
    controllerMigrationRateLimit: 10,
    controlPlaneRequestSigningSecret: "mnemokv-local-demo-controller-secret",
    peers: [],
  }
}

export function defaultClusterPeers(count: number): Peer[] {
  return Array.from({ length: count }, (_, i) => ({
    id: `node-${i + 1}`,
    address: `127.0.0.1:${6381 + i}`,
    apiAddress: `127.0.0.1:${7381 + i}`,
    controlAddress: `127.0.0.1:${7481 + i}`,
  }))
}

export function peersForAutomaticMode(peers: Peer[]): Peer[] {
  const withControlAddresses = peers.map((peer, i) => ({
    ...peer,
    controlAddress: peer.controlAddress.trim() || `127.0.0.1:${7481 + i}`,
  }))

  if (withControlAddresses.length >= 5) {
    return withControlAddresses
  }
  return [...withControlAddresses, ...defaultClusterPeers(5).slice(withControlAddresses.length)]
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
  if (c.mode === "clustered") {
    lines.push(`  id: ${c.clusterId}`)
  }
  lines.push(`  enabled: ${c.mode === "clustered"}`)
  lines.push(`  shardingEnabled: ${c.mode === "clustered"}`)
  lines.push(`  replicationEnabled: ${c.mode === "clustered" && c.replicationEnabled}`)
  if (c.mode === "clustered") {
    lines.push(`  slotCount: ${c.slotCount}`)
    lines.push("  routingMode: proxy")
    lines.push(`  failoverMode: ${c.failoverMode}`)
    if (c.failoverMode === "automatic") {
      lines.push("  controller:")
      lines.push(`    controlPort: ${c.controllerControlPort}`)
      lines.push(`    raftDir: ${c.controllerRaftDir}`)
      lines.push(`    bootstrapNodeId: ${c.controllerBootstrapNodeId}`)
      lines.push(`    observeIntervalMs: ${c.controllerObserveIntervalMs}`)
      lines.push(`    failureTimeoutMs: ${c.controllerFailureTimeoutMs}`)
      lines.push(`    consecutiveFailures: ${c.controllerConsecutiveFailures}`)
      lines.push(`    rebalanceSkewThreshold: ${c.controllerRebalanceSkewThreshold}`)
      lines.push(`    migrationRateLimit: ${c.controllerMigrationRateLimit}`)
    }
    lines.push("  peers:")
    for (const peer of c.peers) {
      lines.push(`    - id: ${peer.id}`)
      lines.push(`      address: ${peer.address}`)
      lines.push(`      apiAddress: ${peer.apiAddress}`)
      if (c.failoverMode === "automatic") {
        lines.push(`      controlAddress: ${peer.controlAddress}`)
        lines.push("      failoverMode: automatic")
      }
    }
  } else {
    lines.push("  peers: []")
  }
  if (c.mode === "clustered" && c.failoverMode === "automatic") {
    lines.push("")
    lines.push("controlPlane:")
    lines.push(`  requestSigningSecret: ${c.controlPlaneRequestSigningSecret}`)
  }
  lines.push("")
  lines.push("persistence:")
  lines.push(`  enabled: ${c.mode === "clustered" && c.failoverMode !== "automatic"}`)
  lines.push(`  dataDir: ${c.dataDir}`)
  lines.push("  snapshotIntervalSec: 60")
  lines.push("  maxSnapshots: 5")
  lines.push(`  loadOnStart: ${c.failoverMode !== "automatic"}`)
  lines.push("  format: json")
  lines.push("")
  lines.push("observability:")
  lines.push(`  apiBindAddr: ${c.apiBindAddr}`)
  lines.push(`  apiPort: ${c.apiPort}`)
  lines.push("  logLevel: info")
  lines.push("")
  return lines.join("\n")
}

export type ValidationError = { field: string; message: string }

export function validate(c: NodeConfig): ValidationError[] {
  const errors: ValidationError[] = []
  if (!c.id.trim()) errors.push({ field: "id", message: "Node id is required" })
  if (c.port < 1 || c.port > 65535) errors.push({ field: "port", message: "Port must be 1-65535" })
  if (c.apiPort < 1 || c.apiPort > 65535) errors.push({ field: "apiPort", message: "API port must be 1-65535" })
  if (c.port === c.apiPort) errors.push({ field: "apiPort", message: "API port must differ from RESP port" })
  if (c.stripeCount < 1) errors.push({ field: "stripeCount", message: "Stripe count must be at least 1" })
  if (c.memoryLimitBytes < 0) errors.push({ field: "memoryLimitBytes", message: "Memory limit cannot be negative" })
  if (c.mode === "clustered") {
    if (!c.clusterId.trim()) errors.push({ field: "clusterId", message: "Cluster id is required" })
    if (c.slotCount < 1 || c.slotCount > 65536) errors.push({ field: "slotCount", message: "Slot count must be 1-65536" })
    if (c.peers.length < 2 || c.peers.length > 5) errors.push({ field: "peers", message: "A cluster needs 2-5 peers" })
    if (!c.peers.some((peer) => peer.id === c.id)) errors.push({ field: "peers", message: "This node's id must appear in the peer list" })
    if (c.peers.some((peer) => !peer.id.trim() || !peer.address.trim() || !peer.apiAddress.trim())) {
      errors.push({ field: "peers", message: "Every peer needs id, RESP address, and API address" })
    }
    if (new Set(c.peers.map((peer) => peer.id)).size !== c.peers.length) {
      errors.push({ field: "peers", message: "Peer ids must be unique" })
    }
    if (new Set(c.peers.map((peer) => peer.address)).size !== c.peers.length) {
      errors.push({ field: "peers", message: "Peer RESP addresses must be unique" })
    }
    if (new Set(c.peers.map((peer) => peer.apiAddress)).size !== c.peers.length) {
      errors.push({ field: "peers", message: "Peer API addresses must be unique" })
    }
    if (c.failoverMode === "automatic") {
      if (c.peers.length < 3) {
        errors.push({ field: "peers", message: "Automatic failover needs at least 3 peers" })
      }
      if (c.peers.some((peer) => !peer.controlAddress.trim())) {
        errors.push({ field: "peers", message: "Every automatic peer needs a controller address" })
      }
      if (new Set(c.peers.map((peer) => peer.controlAddress)).size !== c.peers.length) {
        errors.push({ field: "peers", message: "Peer controller addresses must be unique" })
      }
      if (c.controllerControlPort < 1 || c.controllerControlPort > 65535) {
        errors.push({ field: "controllerControlPort", message: "Controller port must be 1-65535" })
      }
      if (!c.controllerRaftDir.trim()) {
        errors.push({ field: "controllerRaftDir", message: "Controller Raft directory is required" })
      }
      if (!c.controllerBootstrapNodeId.trim()) {
        errors.push({ field: "controllerBootstrapNodeId", message: "Bootstrap node id is required" })
      } else if (!c.peers.some((peer) => peer.id === c.controllerBootstrapNodeId)) {
        errors.push({ field: "controllerBootstrapNodeId", message: "Bootstrap node id must be in the peer list" })
      }
      if (c.controllerObserveIntervalMs <= 0 || c.controllerFailureTimeoutMs <= 0 || c.controllerConsecutiveFailures <= 0) {
        errors.push({ field: "controllerTiming", message: "Controller observation settings must be positive" })
      }
      if (c.controllerRebalanceSkewThreshold <= 0 || c.controllerMigrationRateLimit <= 0) {
        errors.push({ field: "controllerRebalance", message: "Controller rebalance settings must be positive" })
      }
      if (!c.controlPlaneRequestSigningSecret.trim()) {
        errors.push({ field: "controlPlaneRequestSigningSecret", message: "Request signing secret is required" })
      }
    }
  }
  return errors
}
