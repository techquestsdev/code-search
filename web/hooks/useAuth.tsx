"use client";

import { createContext, useContext, type ReactNode } from "react";

interface AuthContextType {
  user: null;
  loading: false;
  authEnabled: false;
  isAdmin: false;
  isAuthenticated: false;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType>({
  user: null,
  loading: false,
  authEnabled: false,
  isAdmin: false,
  isAuthenticated: false,
  logout: async () => {},
});

export function AuthProvider({ children }: { children: ReactNode }) {
  return (
    <AuthContext.Provider
      value={{
        user: null,
        loading: false,
        authEnabled: false,
        isAdmin: false,
        isAuthenticated: false,
        logout: async () => {},
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  return useContext(AuthContext);
}
