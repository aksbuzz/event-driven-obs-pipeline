import { use, useEffect, useTransition } from 'react';
import { useQuery } from 'urql';
import { FilterContext } from '../context/FilterContext';
import { STATS_QUERY } from '../gql/queries';
import { toFromTo } from '../lib/timeRange';
import { EventStats } from '../types';

const POLL_INTERVAL_MS = 30_000;

export function useStats(): { data: EventStats[]; isPending: boolean } {
  const { filter } = use(FilterContext);
  const [isPending, startTransition] = useTransition();

  const variables = {
    ...toFromTo(filter.timeRange),
    service: filter.service ?? undefined,
  };

  const [result, executeQuery] = useQuery({ query: STATS_QUERY, variables });

  useEffect(() => {
    const id = setInterval(() => {
      startTransition(async () => {
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
