import { create } from "zustand"

type AppState = {
  apiBaseUrl: string
  setApiBaseUrl: (url: string) => void
}

const defaultBase =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? "http://127.0.0.1:7380"

const storageKey = "mnemokv.apiBaseUrl"

function initialBaseUrl(): string {
  try {
    const saved = window.localStorage.getItem(storageKey)
    return normalizeBaseUrl(saved || defaultBase)
  } catch {
    return normalizeBaseUrl(defaultBase)
  }
}

function normalizeBaseUrl(url: string): string {
  return url.trim().replace(/\/+$/, "")
}

export const useAppStore = create<AppState>((set) => ({
  apiBaseUrl: initialBaseUrl(),
  setApiBaseUrl: (url) => {
    const normalized = normalizeBaseUrl(url)
    try {
      window.localStorage.setItem(storageKey, normalized)
    } catch {
      // The target still works for this session when storage is unavailable.
    }
    set({ apiBaseUrl: normalized })
  },
}))
