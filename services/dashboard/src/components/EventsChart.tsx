import { useDeferredValue } from 'react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { EventStats } from '../types';
import { transformStats } from '../lib/transformStats';

interface Props {
  stats: EventStats[];
}

export function EventsChart({ stats }: Props) {
  const deferredStats = useDeferredValue(stats, []);
  const data = transformStats(deferredStats);

  return (
    <div className="rounded-lg bg-gray-800 p-4">
      <h2 className="mb-3 text-sm font-medium text-gray-400 uppercase tracking-wide">
        Events / min
      </h2>
      <ResponsiveContainer width="100%" height={200}>
        <AreaChart data={data}>
          <defs>
            <linearGradient id="info" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#60a5fa" stopOpacity={0.4} />
              <stop offset="95%" stopColor="#60a5fa" stopOpacity={0} />
            </linearGradient>
            <linearGradient id="warn" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#fbbf24" stopOpacity={0.4} />
              <stop offset="95%" stopColor="#fbbf24" stopOpacity={0} />
            </linearGradient>
            <linearGradient id="error" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#f87171" stopOpacity={0.4} />
              <stop offset="95%" stopColor="#f87171" stopOpacity={0} />
            </linearGradient>
          </defs>
          <CartesianGrid strokeDasharray="3 3" stroke="#374151" />
          <XAxis
            dataKey="bucket"
            tickFormatter={(v: string) =>
              new Date(v).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
            }
            stroke="#6b7280"
            tick={{ fontSize: 11 }}
          />
          <YAxis stroke="#6b7280" tick={{ fontSize: 11 }} />
          <Tooltip
            contentStyle={{ background: '#1f2937', border: 'none', borderRadius: 6 }}
            labelStyle={{ color: '#9ca3af' }}
            labelFormatter={label =>
              typeof label === 'number' ? new Date(label).toLocaleTimeString() : label
            }
          />
          <Area
            type="monotone"
            dataKey="info"
            stackId="1"
            stroke="#60a5fa"
            fill="url(#info)"
            name="info"
          />
          <Area
            type="monotone"
            dataKey="warn"
            stackId="1"
            stroke="#fbbf24"
            fill="url(#warn)"
            name="warn"
          />
          <Area
            type="monotone"
            dataKey="error"
            stackId="1"
            stroke="#f87171"
            fill="url(#error)"
            name="error"
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
