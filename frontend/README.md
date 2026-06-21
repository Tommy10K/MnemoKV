# MnemoKV Frontend

React/Vite learning, observability, and demonstration UI for MnemoKV.

## Run

Start a backend node from the repository root:

```powershell
go run ./cmd/node -config configs/standalone.yaml
```

Then start the frontend:

```powershell
cd frontend
npm.cmd ci
npm.cmd run dev
```

Open the URL printed by Vite. The default API target is `http://127.0.0.1:7380`; it can be changed
in the Use section and is persisted by the browser. `VITE_API_BASE_URL` sets a different initial
target.

## Features

- Twelve learning chapters with interactive data-structure and eviction visuals.
- YAML configuration generator.
- Live health, engine, metrics, and SSE dashboard.
- HTTP command console and workload command builder.
- Fixed-slot cluster topology and slot-state inspection.
- Runtime eviction-policy controls.
- Go benchmark import plus a built-in deterministic example.
- Runtime validation of backend payloads with distinct malformed and offline states.
- Keyboard, screen-reader, reduced-motion, and responsive-layout support.

## Verify

```powershell
npm.cmd run lint
npm.cmd run build
npm.cmd run test:e2e
```

The Edge end-to-end suite builds and starts an isolated MnemoKV node and frontend server. It covers
live and offline flows, malformed API data, accessibility, and laptop, projector, and tablet sizes.
