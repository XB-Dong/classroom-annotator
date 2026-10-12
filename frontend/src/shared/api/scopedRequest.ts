export class ScopedRequest {
  private scope = '';
  private generation = 0;
  setScope(scope: string) {
    if (this.scope !== scope) {
      this.scope = scope;
      this.generation++;
    }
  }
  async run<T>(
    scope: string,
    request: () => Promise<T>,
    accept: (value: T) => void,
    reject?: (error: unknown) => void,
  ) {
    if (scope !== this.scope) return;
    const captured = ++this.generation;
    try {
      const value = await request();
      if (scope === this.scope && captured === this.generation) accept(value);
    } catch (error) {
      if (scope === this.scope && captured === this.generation) reject?.(error);
    }
  }
}
