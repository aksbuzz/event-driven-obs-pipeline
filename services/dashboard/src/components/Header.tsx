import { use } from 'react';
import { FilterContext } from '../context/FilterContext';
import { TimeRange } from '../types';

const TIME_RANGES: { label: string; value: TimeRange }[] = [
  { label: 'Last 15m', value: '15m' },
  { label: 'Last 1h', value: '1h' },
  { label: 'Last 6h', value: '6h' },
  { label: 'Last 24h', value: '24h' },
];

interface Props {
  services: string[];
  wsStatus: 'connecting' | 'connected' | 'error';
}

export function Header({ services, wsStatus }: Props) {
  const { filter, setFilter } = use(FilterContext);

  const statusColor =
    wsStatus === 'connected'
      ? 'bg-green-500'
      : wsStatus === 'connecting'
        ? 'bg-yellow-400'
        : 'bg-red-500';

  return (
    <header className="flex items-center gap-4 border-b border-gray-700 bg-gray-900 px-6 py-3">
      <span className="text-lg font-semibold text-white">OBS Pipeline</span>

      <select
        className="rounded bg-gray-800 px-2 py-1 text-sm text-gray-200"
        value={filter.service ?? ''}
        onChange={e => setFilter({ ...filter, service: e.target.value || null })}
      >
        <option value="">All services</option>
        {services.map(s => (
          <option key={s} value={s}>
            {s}
          </option>
        ))}
      </select>

      <select
        className="rounded bg-gray-800 px-2 py-1 text-sm text-gray-200"
        value={filter.timeRange}
        onChange={e => setFilter({ ...filter, timeRange: e.target.value as TimeRange })}
      >
        {TIME_RANGES.map(({ label, value }) => (
          <option key={value} value={value}>
            {label}
          </option>
        ))}
      </select>

      <div className="ml-auto flex items-center gap-2">
        <span className={`h-2.5 w-2.5 rounded-full ${statusColor}`} />
        <span className="text-xs text-gray-400 capitalize">{wsStatus}</span>
      </div>
    </header>
  );
}
