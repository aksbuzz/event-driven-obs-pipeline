import { use, useEffect, useMemo, useState, useTransition } from 'react';
import { useQuery } from 'urql';
import { FilterContext } from '../context/FilterContext';
import { ALERTS_QUERY } from '../gql/queries';
import { toFromTo } from '../lib/timeRange';
import { Alert } from '../types';

const POLL_INTERVAL_MS = 30_000;

export function useAlerts(): { data: Alert[]; isPending: boolean } {
  const { filter } = use(FilterContext);
  const [isPending, startTransition] = useTransition();
  const [pollTick, setPollTick] = useState(0);

  // pollTick ensures the `from` timestamp advances on each poll interval.
  const variables = useMemo(
    () => ({ from: toFromTo(filter.timeRange).from, service: filter.service ?? undefined }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [filter.timeRange, filter.service, pollTick],
  );

  const [result, executeQuery] = useQuery({ query: ALERTS_QUERY, variables });

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
    data: result.data?.alerts ?? [],
    isPending: isPending || result.fetching,
  };
}
