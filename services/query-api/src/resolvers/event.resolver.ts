import { IResolvers } from 'mercurius';
import type { IEventService } from '../services/event.service';
import type { EventsFilter } from '../repositories/event.repository';

export function buildEventResolvers(eventService: IEventService): IResolvers {
  return {
    Query: {
      events: (_: unknown, { filter }: { filter?: EventsFilter }) =>
        eventService.getEvents(filter ?? {}),
    },
  };
}
