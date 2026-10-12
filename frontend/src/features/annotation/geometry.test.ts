import { describe, expect, it } from 'vitest';
import { normalizeBox, originalPoint } from './geometry';
describe('original pixel geometry', () => {
  it('maps a scaled pointer to original pixels', () => expect(originalPoint({x:120,y:80},0.5)).toEqual({x:240,y:160}));
  it('normalizes backwards drawing and clips to image bounds', () => expect(normalizeBox([110,80,-120,-50],100,100,true)).toEqual([0,30,100,50]));
  it('rejects only new gestures below two original pixels', () => { expect(normalizeBox([1,1,1,1],100,100,true)).toBeNull(); expect(normalizeBox([1,1,0.5,0.5],100,100)).toEqual([1,1,0.5,0.5]); });
  it('rejects invalid or degenerate geometry', () => { expect(normalizeBox([0,0,NaN,2],100,100)).toBeNull(); expect(normalizeBox([100,0,2,2],100,100)).toBeNull(); });
});
