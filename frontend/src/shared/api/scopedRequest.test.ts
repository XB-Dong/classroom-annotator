import { expect, it } from 'vitest';
import { ScopedRequest } from './scopedRequest';
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((ack) => {
    resolve = ack;
  });
  return { promise, resolve };
}
it('ignores progress from A after B is selected', async () => {
  const guard = new ScopedRequest();
  let shown = 0;
  guard.setScope('A');
  const a = deferred<number>();
  const old = guard.run(
    'A',
    () => a.promise,
    (v) => {
      shown = v;
    },
  );
  guard.setScope('B');
  await guard.run(
    'B',
    async () => 2,
    (v) => {
      shown = v;
    },
  );
  a.resolve(1);
  await old;
  expect(shown).toBe(2);
});
it('ignores an older response after a newer request in the same scope', async () => {
  const guard = new ScopedRequest();
  let shown = 0;
  guard.setScope('A');
  const a = deferred<number>();
  const old = guard.run(
    'A',
    () => a.promise,
    (v) => {
      shown = v;
    },
  );
  await guard.run(
    'A',
    async () => 2,
    (v) => {
      shown = v;
    },
  );
  a.resolve(1);
  await old;
  expect(shown).toBe(2);
});
