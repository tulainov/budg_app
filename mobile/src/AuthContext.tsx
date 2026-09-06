import * as SecureStore from 'expo-secure-store';
import React, { createContext, useContext, useEffect, useMemo, useState } from 'react';

import * as api from './api';

const STORAGE_KEY = 'budget-app-session';

type Session = {
  token: string;
  user: api.User;
  household: api.Household;
};

type AuthContextValue = {
  session: Session | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  signup: (payload: api.SignupPayload) => Promise<void>;
  logout: () => Promise<void>;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    (async () => {
      const stored = await SecureStore.getItemAsync(STORAGE_KEY);
      if (stored) {
        try {
          setSession(JSON.parse(stored));
        } catch {
          await SecureStore.deleteItemAsync(STORAGE_KEY);
        }
      }
      setLoading(false);
    })();
  }, []);

  async function persist(next: Session) {
    setSession(next);
    await SecureStore.setItemAsync(STORAGE_KEY, JSON.stringify(next));
  }

  async function login(email: string, password: string) {
    const res = await api.login(email, password);
    await persist({ token: res.token, user: res.user, household: res.household });
  }

  async function signup(payload: api.SignupPayload) {
    const res = await api.signup(payload);
    await persist({ token: res.token, user: res.user, household: res.household });
  }

  async function logout() {
    setSession(null);
    await SecureStore.deleteItemAsync(STORAGE_KEY);
  }

  const value = useMemo(
    () => ({ session, loading, login, signup, logout }),
    [session, loading]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within an AuthProvider');
  return ctx;
}
