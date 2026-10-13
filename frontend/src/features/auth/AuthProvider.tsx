import {
  createContext,
  useContext,
  useEffect,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from 'react';
import { ApiError, json, request } from '../../shared/api/client';
import type { AuthSession, AuthUser } from '../../shared/api/types';
import { authSession } from './session';
interface AuthContextValue {
  session?: AuthSession;
  loading: boolean;
  error?: string;
  login: (username: string, password: string) => Promise<void>;
  register: (username: string, password: string, inviteCode: string) => Promise<void>;
  logout: () => void;
}
const AuthContext = createContext<AuthContextValue | undefined>(undefined);
export function AuthProvider({ children }: { children: ReactNode }) {
  const session = useSyncExternalStore(authSession.subscribe, authSession.getSnapshot);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>();
  useEffect(() => {
    let active = true;
    void request<AuthSession>('/auth/me')
      .then((result) => {
        if (active) authSession.activate(result);
      })
      .catch((cause) => {
        if (active && !(cause instanceof ApiError && cause.status === 401))
          setError((cause as Error).message);
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);
  async function login(username: string, password: string) {
    const result = await request<AuthSession>('/auth/login', json({ username, password }));
    authSession.activate(result);
    setError(undefined);
  }
  async function register(username: string, password: string, inviteCode: string) {
    await request<{ user: AuthUser }>('/auth/register', json({ username, password, inviteCode }));
  }
  function logout() {
    const pending = request<void>('/auth/logout', json({}));
    authSession.clear();
    void pending.catch((cause) => {
      if (!(cause instanceof ApiError && cause.code === 'STALE_AUTH_RESPONSE'))
        setError('退出请求未确认，请关闭页面并联系管理员。');
    });
  }
  return (
    <AuthContext.Provider value={{ session, loading, error, login, register, logout }}>
      {children}
    </AuthContext.Provider>
  );
}
export function useAuth() {
  const context = useContext(AuthContext);
  if (!context) throw new Error('AuthProvider is required');
  return context;
}
