import { Link } from "react-router-dom"

const features = [
  "RESP2 protocol over TCP — works with redis-cli and any RESP client",
  "Strings, lists, sorted sets with O(1) and O(log n) operations",
  "Lock striping for concurrent access",
  "Memory limits with FIFO, LRU, LFU, and Random eviction",
  "Fixed-slot cluster routing through any node",
  "Synchronous leader-to-replica write acknowledgement",
  "Manual repair or opt-in automatic recovery with stale-node fencing",
]

export function HomePage() {
  return (
    <div className="flex flex-col gap-12 py-8">
      <section className="flex flex-col gap-4">
        <h1 className="text-4xl font-semibold tracking-tight text-white">MnemoKV</h1>
        <p className="max-w-2xl text-lg text-[#9ca3af]">
          An educational in-memory distributed key-value store written in Go. Built to teach the
          algorithms, data structures, and trade-offs that production systems rely on.
        </p>
        <p className="max-w-2xl rounded-md border border-sky-500/40 bg-sky-500/10 p-3 text-sm text-sky-200">
          Standalone mode, snapshots, and the fixed-slot cluster path share the same command
          engine. Cluster failover is a startup choice: manual operator repair or the embedded
          automatic recovery controller.
        </p>
      </section>

      <section className="flex flex-col gap-3">
        <h2 className="text-sm font-semibold uppercase tracking-wider text-[#8b949e]">
          What it does
        </h2>
        <ul className="grid grid-cols-1 gap-2 text-sm text-[#e6edf3] md:grid-cols-2">
          {features.map((feature) => (
            <li key={feature} className="flex items-start gap-2">
              <span className="mt-1.5 inline-block size-1.5 shrink-0 rounded-full bg-emerald-400" />
              <span>{feature}</span>
            </li>
          ))}
        </ul>
      </section>

      <section className="grid grid-cols-1 gap-4 md:grid-cols-2">
        <HomeCard
          to="/learn"
          title="Learn"
          description="Twelve chapters covering in-memory databases, RESP, data structures, concurrency, eviction policies, sharding, replication, gossip, and failover."
        />
        <HomeCard
          to="/use"
          title="Use the Database"
          description="Configure a node, start it, watch live metrics, run workloads, visualize cluster topology, and compare benchmarks."
        />
      </section>
    </div>
  )
}

type HomeCardProps = {
  to: string
  title: string
  description: string
}

function HomeCard({ to, title, description }: HomeCardProps) {
  return (
    <Link
      to={to}
      className="group flex flex-col gap-2 rounded-lg border border-[#1f2937] bg-[#0b0f17] p-5 transition-colors hover:border-emerald-500/40 hover:bg-[#111722]"
    >
      <h3 className="text-lg font-semibold text-white group-hover:text-emerald-400">{title} →</h3>
      <p className="text-sm text-[#9ca3af]">{description}</p>
    </Link>
  )
}
