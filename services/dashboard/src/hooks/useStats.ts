import { use, useEffect, useMemo, useState, useTransition } from 'react';
import { useQuery } from 'urql';
import { FilterContext } from '../context/FilterContext';
import { STATS_QUERY } from '../gql/queries';
import { toFromTo } from '../lib/timeRange';
import { EventStats } from '../types';

const POLL_INTERVAL_MS = 30_000;

export function useStats(): { data: EventStats[]; isPending: boolean } {
  const { filter } = use(FilterContext);
  const [isPending, startTransition] = useTransition();
  const [pollTick, setPollTick] = useState(0);

  // pollTick ensures from/to advance on each poll interval.
  const variables = useMemo(
    () => ({ ...toFromTo(filter.timeRange), service: filter.service ?? undefined }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [filter.timeRange, filter.service, pollTick],
  );

  const [result, executeQuery] = useQuery({ query: STATS_QUERY, variables });

  useEffect(() => {
    const id = setInterval(() => {
      startTransition(async () => {
        setPollTick(t => t + 1);
        executeQuery({ requestPolicy: 'network-only' });
      });
    }, POLL_INTERVAL_MS);
    return () => clearInterval(id);
  }, [executeQuery]);

  return {
    data: result.data?.stats ?? [],
    isPending: isPending || result.fetching,
  };
}
