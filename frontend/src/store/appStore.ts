import { create } from "zustand"

type AppState = {
  apiBaseUrl: string
  setApiBaseUrl: (url: string) => void
}

const defaultBase =
  (import.meta.env.VITE_API_BASE_URL as string | undefined) ?? "http://127.0.0.1:7380"

export const useAppStore = create<AppState>((set) => ({
  apiBaseUrl: defaultBase,
  setApiBaseUrl: (url) => set({ apiBaseUrl: url.replace(/\/+$/, "") }),
}))
