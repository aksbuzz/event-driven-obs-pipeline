interface Props {
  totalEvents: number;
  errorRate: number;
  activeServices: number;
  isPending: boolean;
}

export function StatCards({ totalEvents, errorRate, activeServices, isPending }: Props) {
  const opacity = isPending ? 'opacity-60' : 'opacity-100';

  return (
    <div className={`grid grid-cols-3 gap-4 transition-opacity ${opacity}`}>
      <Card label="Events / hr" value={totalEvents.toLocaleString()} />
      <Card label="Error Rate" value={`${errorRate.toFixed(1)}%`} highlight={errorRate > 5} />
      <Card label="Active Services" value={String(activeServices)} />
    </div>
  );
}

function Card({
  label,
  value,
  highlight = false,
}: {
  label: string;
  value: string;
  highlight?: boolean;
}) {
  return (
    <div className="rounded-lg bg-gray-800 p-4">
      <p className="text-xs text-gray-400 uppercase tracking-wide">{label}</p>
      <p className={`mt-1 text-2xl font-bold ${highlight ? 'text-red-400' : 'text-white'}`}>
        {value}
      </p>
    </div>
  );
}
