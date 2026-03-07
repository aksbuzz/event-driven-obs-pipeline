import { use } from 'react';
import { LiveEvent } from '../types';
import { FilterContext } from '../context/FilterContext';
import { useSubscription } from 'urql';
import { EVENT_SUBSCRIPTION } from '../gql/queries';
import { addToRingBuffer } from '../lib/ringBuffer';

const RING_SIZE = 200;

export function useLiveEvents(): LiveEvent[] {
  const { filter } = use(FilterContext);

  const [result] = useSubscription(
    {
      query: EVENT_SUBSCRIPTION,
      variables: { service: filter.service ?? undefined },
    },
    (prev: LiveEvent[] = [], response: { eventReceived: LiveEvent }) =>
      addToRingBuffer(prev, response.eventReceived, RING_SIZE),
  );

  return result.data ?? [];
}
