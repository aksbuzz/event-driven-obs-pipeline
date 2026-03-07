export function addToRingBuffer<T>(prev: T[], item: T, maxSize: number): T[] {
  const next = [item, ...prev];
  return next.length > maxSize ? next.slice(0, maxSize) : next;
}
