import { create } from "zustand";
import { storeIPv6Passwords } from "../ipv6/proxyLinks";

const TOKEN_KEY = "resin_admin_token";

function loadInitialToken(): string {
  if (typeof window === "undefined") {
    return "";
  }
  return window.localStorage.getItem(TOKEN_KEY) ?? "";
}

type AuthState = {
  token: string;
  setToken: (token: string) => void;
  clearToken: () => void;
};

export const useAuthStore = create<AuthState>((set, get) => ({
  token: loadInitialToken(),
  setToken: (token) => {
    const next = token.trim();
    if (next !== get().token) {
      storeIPv6Passwords(new Map());
    }
    if (typeof window !== "undefined") {
      window.localStorage.setItem(TOKEN_KEY, next);
    }
    set({ token: next });
  },
  clearToken: () => {
    storeIPv6Passwords(new Map());
    if (typeof window !== "undefined") {
      window.localStorage.removeItem(TOKEN_KEY);
    }
    set({ token: "" });
  },
}));

export function getStoredAuthToken(): string {
  return useAuthStore.getState().token;
}
