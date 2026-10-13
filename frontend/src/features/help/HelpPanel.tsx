import { useEffect, useState, type FormEvent } from 'react';
import { json, request } from '../../shared/api/client';
import type { HelpAnswer, HelpDocument } from '../../shared/api/types';
import { useAuth } from '../auth/AuthProvider';
import { authSession } from '../auth/session';
import { DocumentView, safeSectionId } from './DocumentView';
export function HelpPanel({ close }: { close: () => void }) {
  const { session } = useAuth();
  const [document, setDocument] = useState<HelpDocument>();
  const [question, setQuestion] = useState('');
  const [answer, setAnswer] = useState<HelpAnswer>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const [documentError, setDocumentError] = useState<string>();
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    let active = true;
    setAnswer(undefined);
    setError(undefined);
    setQuestion('');
    setBusy(false);
    void request<HelpDocument>('/help/document')
      .then((result) => {
        if (active) {
          setDocument(result);
          setDocumentError(undefined);
        }
      })
      .catch((cause) => {
        if (active) setDocumentError((cause as Error).message);
      });
    return () => {
      active = false;
    };
  }, [session?.user.id, retry]);
  function navigate(sectionId: string) {
    if (
      !safeSectionId(sectionId) ||
      !document?.sections.some((section) => section.id === sectionId)
    )
      return;
    const heading = window.document.getElementById(sectionId);
    heading?.scrollIntoView?.({ block: 'start', behavior: 'auto' });
    heading?.focus({ preventScroll: true });
  }
  async function ask(event: FormEvent) {
    event.preventDefault();
    if (!session || busy || !question.trim()) return;
    const generation = authSession.generation;
    setBusy(true);
    setError(undefined);
    setAnswer(undefined);
    try {
      const result = await request<HelpAnswer>(
        '/help/questions',
        json({ question: question.trim() }),
      );
      if (generation === authSession.generation) setAnswer(result);
    } catch (cause) {
      if (generation === authSession.generation) setError((cause as Error).message);
    } finally {
      if (generation === authSession.generation) setBusy(false);
    }
  }
  return (
    <aside className="help-shell" id="help-panel" aria-label="使用指南">
      <div className="panel-title">
        <div>
          <span className="eyebrow">随手查一查</span>
          <h2>使用指南</h2>
        </div>
        <button onClick={close} aria-label="收起使用指南">
          收起
        </button>
      </div>
      <p className="muted">查阅完整手册，或提出一个平台操作问题。</p>
      {documentError && (
        <p role="alert" className="notice error">
          手册加载失败：{documentError}{' '}
          <button onClick={() => setRetry((value) => value + 1)}>重试</button>
        </p>
      )}
      {document && (
        <>
          <p className="muted">手册版本 {document.documentVersion}</p>
          <nav className="manual-sections" aria-label="手册章节">
            {document.sections
              .filter((section) => safeSectionId(section.id))
              .map((section) => (
                <button key={section.id} onClick={() => navigate(section.id)}>
                  {section.title}
                </button>
              ))}
          </nav>
        </>
      )}
      <form className="help-question" onSubmit={(event) => void ask(event)}>
        <label>
          操作问题
          <textarea
            aria-label="操作问题"
            maxLength={1000}
            rows={3}
            value={question}
            disabled={!session || busy}
            onChange={(event) => setQuestion(event.target.value)}
            placeholder={session ? '例如：如何处理版本冲突？' : '登录后可使用文档问答'}
          />
        </label>
        <p className="muted">只问平台操作，请勿输入密码、邀请码或公司业务信息。</p>
        <button
          type="submit"
          className="primary full"
          disabled={!session || busy || !question.trim() || !document}
        >
          {busy ? '正在依据手册回答…' : '依据手册回答'}
        </button>
      </form>
      {error && (
        <p className="notice error" role="alert">
          {error} 完整手册仍可查阅。
        </p>
      )}
      {answer && (
        <section className="help-answer" aria-label="手册回答">
          <p>{answer.answer}</p>
          <div className="manual-sections">
            {answer.references
              .filter((reference) =>
                document?.sections.some(
                  (section) =>
                    section.id === reference.sectionId && section.anchor === reference.anchor,
                ),
              )
              .map((reference) => (
                <button key={reference.sectionId} onClick={() => navigate(reference.sectionId)}>
                  参见：{reference.title}
                </button>
              ))}
          </div>
        </section>
      )}
      {document ? (
        <DocumentView document={document} />
      ) : (
        !documentError && <p role="status">正在加载完整手册…</p>
      )}
    </aside>
  );
}
