import { Alert } from '../types';

const SEVERITY_BADGE: Record<string, string> = {
  critical: 'bg-red-900 text-red-300',
  warning: 'bg-yellow-900 text-yellow-300',
};

interface Props {
  alerts: Alert[];
}

export function AlertsTable({ alerts }: Props) {
  return (
    <div className="rounded-lg bg-gray-800 p-4 flex flex-col">
      <h2 className="mb-3 text-sm font-medium text-gray-400 uppercase tracking-wide">Alerts</h2>
      <div className="overflow-y-auto flex-1 max-h-72">
        {alerts.length === 0 ? (
          <p className="text-xs text-gray-500 py-4 text-center">No alerts</p>
        ) : (
          <table className="w-full text-xs">
            <thead>
              <tr className="text-gray-500 text-left border-b border-gray-700">
                <th className="pb-1 pr-3">Time</th>
                <th className="pb-1 pr-3">Service</th>
                <th className="pb-1 pr-3">Rule</th>
                <th className="pb-1">Severity</th>
              </tr>
            </thead>
            <tbody>
              {alerts.map(a => (
                <tr key={a.alertId} className="border-b border-gray-700/50">
                  <td className="py-1 pr-3 text-gray-400">
                    {new Date(a.firedAt).toLocaleTimeString()}
                  </td>
                  <td className="py-1 pr-3 text-gray-200">{a.service}</td>
                  <td className="py-1 pr-3 text-gray-400">{a.rule}</td>
                  <td className="py-1">
                    <span
                      className={`rounded px-1.5 py-0.5 text-[10px] font-semibold ${SEVERITY_BADGE[a.severity] ?? 'bg-gray-700 text-gray-300'}`}
                    >
                      {a.severity}
                    </span>
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
