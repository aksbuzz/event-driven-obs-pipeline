import { injectable, inject } from 'inversify';
import { TYPES } from '../types';
import type { IEventRepository, GqlEvent, EventsFilter } from '../repositories/event.repository';

export interface IEventService {
  getEvents(filter: EventsFilter): Promise<GqlEvent[]>;
}

@injectable()
export class EventService implements IEventService {
  constructor(@inject(TYPES.EventRepository) private eventRepo: IEventRepository) {}

  getEvents(filter: EventsFilter): Promise<GqlEvent[]> {
    return this.eventRepo.listEvents(filter);
  }
}
