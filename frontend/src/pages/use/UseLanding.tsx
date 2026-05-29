export function UseLanding() {
  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-3xl font-semibold tracking-tight text-white">Use the Database</h1>
      <p className="max-w-2xl text-[#9ca3af]">
        This section will let you generate a configuration file, start the database, monitor it
        live, run workloads, visualize the cluster, and compare benchmarks. It is built in later
        phases and connects to the backend's HTTP and SSE APIs.
      </p>
      <p className="text-sm text-[#6b7280]">
        Coming next: the configuration page (Phase F2.1).
      </p>
    </div>
  )
}
