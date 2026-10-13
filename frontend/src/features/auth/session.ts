import type { AuthSession } from '../../shared/api/types';

class SessionScope {
  current?: AuthSession;
  generation = 0;
  private listeners = new Set<() => void>();
  private requests = new Set<AbortController>();
  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };
  getSnapshot = () => this.current;
  private transition(session?: AuthSession) {
    this.generation++;
    this.requests.forEach((controller) => controller.abort());
    this.requests.clear();
    this.current = session;
    this.listeners.forEach((listener) => listener());
  }
  activate(session: AuthSession) {
    this.transition(session);
  }
  clear() {
    this.transition();
  }
  track(controller: AbortController) {
    this.requests.add(controller);
    return () => {
      this.requests.delete(controller);
    };
  }
}
export const authSession = new SessionScope();
