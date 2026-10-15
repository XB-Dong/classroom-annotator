import { useState, type FormEvent } from 'react';
import { useAuth } from './AuthProvider';
export function AuthPage() {
  const auth = useAuth();
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [inviteCode, setInviteCode] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [notice, setNotice] = useState<string>();
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError(undefined);
    setNotice(undefined);
    try {
      if (mode === 'register') {
        await auth.register(username, password, inviteCode);
        setInviteCode('');
        setMode('login');
        setNotice('账号已创建，请登录进入工作区。');
      } else await auth.login(username, password);
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <main className="auth-layout">
      <section className="auth-card" aria-labelledby="auth-title">
        <h2 id="auth-title">{mode === 'login' ? '登录工作区' : '注册账号'}</h2>
        {mode === 'register' && <p className="muted">注册需要管理员提供的邀请码。</p>}
        <div className="tabs" aria-label="账号入口">
          <button
            className={mode === 'login' ? 'active' : ''}
            disabled={busy}
            onClick={() => {
              setMode('login');
              setError(undefined);
            }}
          >
            登录账号
          </button>
          <button
            className={mode === 'register' ? 'active' : ''}
            disabled={busy}
            onClick={() => {
              setMode('register');
              setError(undefined);
            }}
          >
            注册账号
          </button>
        </div>
        <form onSubmit={(event) => void submit(event)}>
          <label>
            用户名
            <input
              aria-label="用户名"
              name="username"
              autoComplete="username"
              required
              minLength={3}
              maxLength={32}
              pattern={'[A-Za-z0-9_\\-]{3,32}'}
              value={username}
              disabled={busy}
              onChange={(event) => setUsername(event.target.value)}
            />
            {mode === 'register' && <small>3–32 位英文字母、数字、下划线或短横线。</small>}
          </label>
          <label>
            密码
            <input
              aria-label="密码"
              name="password"
              type="password"
              autoComplete={mode === 'register' ? 'new-password' : 'current-password'}
              required
              value={password}
              disabled={busy}
              onChange={(event) => setPassword(event.target.value)}
            />
            {mode === 'register' && <small>12–128 字节；中文字符会占多个字节。</small>}
          </label>
          {mode === 'register' && (
            <label>
              邀请码
              <input
                name="inviteCode"
                autoComplete="off"
                required
                value={inviteCode}
                disabled={busy}
                onChange={(event) => setInviteCode(event.target.value)}
              />
            </label>
          )}
          {(error || auth.error) && (
            <p className="notice error" role="alert">
              {error || auth.error}
            </p>
          )}
          {notice && (
            <p className="notice" role="status">
              {notice}
            </p>
          )}
          <button className="primary full" disabled={busy || auth.loading} type="submit">
            {busy ? '正在提交…' : mode === 'register' ? '创建账号' : '登录工作区'}
          </button>
        </form>
      </section>
    </main>
  );
}
