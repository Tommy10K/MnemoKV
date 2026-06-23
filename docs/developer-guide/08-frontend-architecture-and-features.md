# 8. React Frontend Architecture And Features

## Frontend Role

The frontend is a learning, observability, configuration, and demonstration layer. It does not own
database state and is not required for the backend. It talks to one selected node through HTTP and
SSE.

The stack is React 19, TypeScript, Vite, React Router, Zustand, Recharts, React Flow, and Tailwind
through PostCSS.

## Bootstrap And Routing

[`frontend/src/main.tsx`](../../frontend/src/main.tsx) mounts React in `StrictMode`.
[`frontend/src/app/App.tsx`](../../frontend/src/app/App.tsx) provides `BrowserRouter`.
[`frontend/src/app/routes.tsx`](../../frontend/src/app/routes.tsx) is the route map.

The three nested layouts are:

- `MainLayout`: global header, skip link, primary navigation, route focus, and footer.
- `LearnLayout`: chapter navigation and educational content outlet.
- `UseLayout`: operational navigation, API target selector, and tool outlet.

Operational pages are lazy-loaded to reduce the initial bundle. The learning chapter registry under
[`pages/learn/chapters`](../../frontend/src/pages/learn/chapters/) keeps chapter metadata and content
in one ordered collection.

## API Target State

[`frontend/src/store/appStore.ts`](../../frontend/src/store/appStore.ts) is the only genuinely global
client state. Zustand stores a normalized API base URL, initializes it from `VITE_API_BASE_URL` or
the standalone default, and persists user changes in `localStorage`.

API functions read the current store value at request time, so changing the target affects polling,
commands, and policy operations without a page reload.

## Typed And Runtime-Validated Boundaries

[`frontend/src/api/types.ts`](../../frontend/src/api/types.ts) declares expected TypeScript shapes.
TypeScript alone cannot validate network JSON, so
[`frontend/src/api/validate.ts`](../../frontend/src/api/validate.ts) checks every HTTP and SSE
payload at runtime.

[`frontend/src/api/client.ts`](../../frontend/src/api/client.ts) owns HTTP requests and parser calls.
[`frontend/src/api/events.ts`](../../frontend/src/api/events.ts) owns `EventSource`. Components do
not cast arbitrary responses.

The UI distinguishes:

- unavailable transport or node;
- non-success HTTP status;
- successful HTTP with an unexpected response contract;
- valid command-level error results.

## Hooks And Live Data

- [`useNodeStatus`](../../frontend/src/hooks/useNodeStatus.ts) polls health every five seconds and
  cancels in-flight work when the target changes.
- [`useNodeEvents`](../../frontend/src/hooks/useNodeEvents.ts) manages SSE state, detects stale
  streams, retains 60 chart points, and derives throughput from `cmd.total` deltas.
- [`useClusterState`](../../frontend/src/hooks/useClusterState.ts) polls authoritative metadata and
  records the last 20 observed metadata-version changes.

Each hook uses effect cleanup to cancel timers, abort fetches, or close `EventSource`. React
`StrictMode` intentionally exercises this cleanup twice in development.

## Operational Pages

| Page | Implementation and behavior |
| --- | --- |
| Configure | `ConfigPage.tsx` plus `lib/config.ts`; validates and downloads YAML but does not reconfigure a process |
| Dashboard | polls health/state, consumes SSE, and renders accessible memory/throughput summaries |
| Console | tokenizes quoted commands, calls `/commands`, and renders recursive RESP-shaped results |
| Workloads | builds a reproducible `cmd/workload` invocation; it does not launch processes |
| Cluster | renders membership hints, authoritative slots, roles, terms, sequences, and metadata history |
| Eviction Lab | changes the live policy and displays memory/eviction state |
| Benchmarks | parses Go benchmark text or JSON and charts latency, bytes, and allocations |

The built-in benchmark is [`frontend/public/examples/engine-bench.txt`](../../frontend/public/examples/engine-bench.txt).
The reusable parser is [`frontend/src/lib/benchmark.ts`](../../frontend/src/lib/benchmark.ts).

## Learning Features

The twelve chapters move from in-memory concepts and RESP through value structures, lock striping,
eviction, sharding, replication, membership, failover, and benchmarks. Interactive visuals under
[`frontend/src/components/visuals`](../../frontend/src/components/visuals/) illustrate linked lists,
skip lists, stripes, and eviction.

Learning text must distinguish general distributed-systems theory from MnemoKV's actual
implementation. Manual mode uses local heartbeat observations without election. The separate
five-node automatic mode uses a Raft-backed controller for recovery decisions, while membership
hints still never change ownership by themselves. The Cluster page renders the backend's recovery
state, one-copy/unavailable slots, active-plan progress, and degraded-window warning.

## Accessibility And Responsive Design

The app includes a skip link, visible focus, semantic headings and navigation, labelled controls,
live regions, keyboard-accessible scroll areas, text summaries for charts/topology, and reduced
motion styles. Responsive tests cover laptop, projector, and tablet viewports.

[`frontend/e2e/app.spec.ts`](../../frontend/e2e/app.spec.ts) uses Playwright and Axe to verify these
properties together with live/offline behavior.

## React Guidelines For Changes

- Keep network access in `src/api` and lifecycle logic in hooks.
- Prefer local state unless several unrelated routes truly share it.
- Keep response types explicit and validate unknown data before narrowing.
- Clean up effects; development `StrictMode` will reveal missing cleanup.
- Preserve usable offline states and route-level keyboard focus.
- Update learning claims when backend behavior changes.
