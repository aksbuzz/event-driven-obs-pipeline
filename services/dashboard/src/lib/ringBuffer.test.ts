import { describe, expect, it } from 'vitest';
import { addToRingBuffer } from './ringBuffer';

describe('addToRingBuffer', () => {
  it('prepends new item', () => {
    const result = addToRingBuffer([], 'a', 5);
    expect(result).toEqual(['a']);
  });

  it('keeps newest items at front', () => {
    const result = addToRingBuffer(['b'], 'a', 5);
    expect(result[0]).toBe('a');
    expect(result[1]).toBe('b');
  });

  it('caps at max size', () => {
    const existing = ['b', 'c', 'd'];
    const result = addToRingBuffer(existing, 'a', 3);
    expect(result).toHaveLength(3);
    expect(result[0]).toBe('a');
    expect(result[2]).toBe('c');
  });

  it('does not mutate input array', () => {
    const existing = ['b'];
    addToRingBuffer(existing, 'a', 5);
    expect(existing).toHaveLength(1);
  });
});
