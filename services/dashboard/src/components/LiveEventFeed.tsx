import { useEffect, useRef } from 'react';
import { LiveEvent } from '../types';

const LEVEL_COLOR: Record<string, string> = {
  info: 'text-blue-400',
  warn: 'text-yellow-400',
  error: 'text-red-400',
  debug: 'text-gray-400',
};

interface Props {
  events: LiveEvent[];
}

export function LiveEventFeed({ events }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (containerRef.current) {
      containerRef.current.scrollTop = 0;
    }
  }, [events.length]);

  return (
    <div className="rounded-lg bg-gray-800 p-4 flex flex-col">
      <h2 className="mb-3 text-sm font-medium text-gray-400 uppercase tracking-wide">
        Live Event Feed
      </h2>
      <div ref={containerRef} className="overflow-y-auto flex-1 max-h-72">
        {events.length === 0 ? (
          <p className="text-xs text-gray-500 py-4 text-center">Waiting for events…</p>
        ) : (
          <table className="w-full text-xs">
            <thead>
              <tr className="text-gray-500 text-left border-b border-gray-700">
                <th className="pb-1 pr-3">Time</th>
                <th className="pb-1 pr-3">Service</th>
                <th className="pb-1 pr-3">Env</th>
                <th className="pb-1">Level</th>
              </tr>
            </thead>
            <tbody>
              {events.map(e => (
                <tr key={e.eventId} className="border-b border-gray-700/50">
                  <td className="py-1 pr-3 text-gray-400">
                    {new Date(e.timestamp).toLocaleTimeString()}
                  </td>
                  <td className="py-1 pr-3 text-gray-200">{e.service}</td>
                  <td className="py-1 pr-3 text-gray-400">{e.environment}</td>
                  <td className={`py-1 font-medium ${LEVEL_COLOR[e.level] ?? 'text-gray-300'}`}>
                    {e.level}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
