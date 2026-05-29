import type { NodeEvent } from "./types"

export type EventsHandlers = {
  onEvent: (e: NodeEvent) => void
  onOpen?: () => void
  onError?: () => void
}

// connectEvents opens an SSE connection to /events on the given base URL and
// forwards parsed payloads to onEvent. The returned function closes it.
// EventSource handles reconnection itself when the browser allows it; for
// hard failures the consumer can call close() and reconnect on its own
// schedule.
export function connectEvents(baseUrl: string, handlers: EventsHandlers): () => void {
  const src = new EventSource(baseUrl + "/events")
  src.onopen = () => handlers.onOpen?.()
  src.onerror = () => handlers.onError?.()
  src.onmessage = (msg) => {
    try {
      const parsed = JSON.parse(msg.data) as NodeEvent
      handlers.onEvent(parsed)
    } catch {
      // ignore malformed payload
    }
  }
  return () => src.close()
}
