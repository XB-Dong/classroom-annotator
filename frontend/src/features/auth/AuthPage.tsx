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
      <section className="auth-intro">
        <span className="eyebrow">课堂手账 / ANNOTATION WORKSPACE</span>
        <h2>
          把每一个人头，
          <br />
          认真框好。
        </h2>
        <p>
          从素材到抽帧，从人工标注到复核。
          <br />
          在同一个工作区里，让每次修改都有据可查。
        </p>
        <div className="workflow-note">
          <span>01 选素材</span>
          <span>02 标注与复核</span>
          <span>03 导出数据</span>
        </div>
      </section>
      <section className="auth-card" aria-labelledby="auth-title">
        <h2 id="auth-title">{mode === 'login' ? '欢迎回到工作台' : '加入共享工作区'}</h2>
        <p className="muted">
          {mode === 'login' ? '登录后继续你的标注与复核。' : '向管理员获取邀请码，创建自己的账号。'}
        </p>
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
              pattern="[A-Za-z0-9_-]{3,32}"
              value={username}
              disabled={busy}
              onChange={(event) => setUsername(event.target.value)}
            />
            <small>3–32 位英文字母、数字、下划线或短横线。</small>
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
            <small>12–128 字节；中文字符会占多个字节。</small>
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
        <p className="auth-footer">需要操作说明？顶部“使用指南”可直接查阅手册。</p>
      </section>
    </main>
  );
}
