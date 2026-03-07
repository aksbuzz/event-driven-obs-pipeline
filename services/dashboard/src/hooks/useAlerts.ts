import { use, useEffect, useTransition } from 'react';
import { useQuery } from 'urql';
import { FilterContext } from '../context/FilterContext';
import { ALERTS_QUERY } from '../gql/queries';
import { toFromTo } from '../lib/timeRange';
import { Alert } from '../types';

const POLL_INTERVAL_MS = 30_000;

export function useAlerts(): { data: Alert[]; isPending: boolean } {
  const { filter } = use(FilterContext);
  const [isPending, startTransition] = useTransition();

  const { from } = toFromTo(filter.timeRange);
  const variables = {
    from,
    service: filter.service ?? undefined,
  };

  const [result, executeQuery] = useQuery({ query: ALERTS_QUERY, variables });

  useEffect(() => {
    const id = setInterval(() => {
      startTransition(async () => {
        executeQuery({ requestPolicy: 'network-only' });
      });
    }, POLL_INTERVAL_MS);
    return () => clearInterval(id);
  }, [executeQuery]);

  return {
    data: result.data?.alerts ?? [],
    isPending: isPending || result.fetching,
  };
}
