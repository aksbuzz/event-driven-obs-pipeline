export const TYPES = {
  Pool: Symbol.for('Pool'),
  EventRepository: Symbol.for('EventRepository'),
  AlertRepository: Symbol.for('AlertRepository'),
  StatsRepository: Symbol.for('StatsRepository'),
  EventService: Symbol.for('EventService'),
  StatsService: Symbol.for('StatsService'),
  AlertService: Symbol.for('AlertService'),
};

export interface PubSubEmitter {
  emit(msg: { topic: string; [k: string]: unknown }, cb: (err?: Error) => void): void;
  on(
    topic: string,
    handler: (msg: unknown, done: () => void) => void,
    cb?: (err?: Error) => void,
  ): void;
  removeListener(topic: string, handler: Function, cb?: (err?: Error) => void): void;
  close(cb?: (err?: Error) => void): void;
}