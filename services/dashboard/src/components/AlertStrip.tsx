import { useState } from 'react';
import { Alert } from '../types';

const ACTIVE_WINDOW_MS = 5 * 60 * 1000;

interface Props {
  alerts: Alert[];
}

export function AlertStrip({ alerts }: Props) {
  const [dismissed, setDismissed] = useState<Set<string>>(new Set());

  const now = Date.now();
  const active = alerts.filter(
    a => now - new Date(a.firedAt).getTime() < ACTIVE_WINDOW_MS && !dismissed.has(a.alertId),
  );

  if (active.length === 0) return null;

  return (
    <div className="flex flex-col gap-1 px-6 py-2">
      {active.map(a => (
        <div
          key={a.alertId}
          className={`flex items-center justify-between rounded px-4 py-2 text-sm ${
            a.severity === 'critical' ? 'bg-red-900 text-red-200' : 'bg-yellow-900 text-yellow-200'
          }`}
        >
          <span>
            <strong>{a.service}</strong>: {a.rule} — {a.severity}
            {' · '}
            {new Date(a.firedAt).toLocaleTimeString()}
          </span>
          <button
            onClick={() => setDismissed(prev => new Set([...prev, a.alertId]))}
            className="ml-4 text-xs opacity-70 hover:opacity-100"
          >
            ✕
          </button>
        </div>
      ))}
    </div>
  );
}
