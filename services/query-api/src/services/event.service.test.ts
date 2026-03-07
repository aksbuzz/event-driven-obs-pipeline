import { describe, it, expect, vi } from 'vitest';
import { EventService } from './event.service';
import type { IEventRepository, GqlEvent } from '../repositories/event.repository';

const makeRepo = (events: GqlEvent[] = []): IEventRepository => ({
  listEvents: vi.fn().mockResolvedValue(events),
});

describe('EventService.getEvents', () => {
  it('passes filter through to repository', async () => {
    const repo = makeRepo();
    const svc = new EventService(repo);

    await svc.getEvents({ service: 'payments-api', level: 'error' });

    expect(repo.listEvents).toHaveBeenCalledWith(
      expect.objectContaining({ service: 'payments-api', level: 'error' }),
    );
  });

  it('returns events from repository', async () => {
    const events: GqlEvent[] = [
      {
        eventId: 'x',
        service: 'svc',
        environment: 'prod',
        instance: null,
        serviceVersion: null,
        level: 'info',
        category: null,
        timestamp: '2026-03-06T10:00:00Z',
        payload: null,
        enrichment: null,
        ingestedAt: '2026-03-06T10:00:01Z',
      },
    ];
    const repo = makeRepo(events);
    const svc = new EventService(repo);

    const result = await svc.getEvents({});

    expect(result).toHaveLength(1);
    expect(result[0].eventId).toBe('x');
  });
});
