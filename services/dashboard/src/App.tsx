import { useState } from 'react';
import { Provider } from 'urql';
import { AlertStrip } from './components/AlertStrip';
import { AlertsTable } from './components/AlertsTable';
import { EventsChart } from './components/EventsChart';
import { Header } from './components/Header';
import { LiveEventFeed } from './components/LiveEventFeed';
import { StatCards } from './components/StatCards';
import { FilterContext } from './context/FilterContext';
import { urqlClient } from './gql/client';
import { useAlerts } from './hooks/useAlerts';
import { useLiveEvents } from './hooks/useLiveEvents';
import { useStats } from './hooks/useStats';
import { deriveMetrics } from './lib/deriveMetrics';
import { FilterState } from './types';

function Dashboard() {
  const { data: stats, isPending: statsPending } = useStats();
  const { data: alerts } = useAlerts();
  const events = useLiveEvents();

  const metrics = deriveMetrics(stats);
  const services = [...new Set(stats.map(s => s.service))].sort();

  // Derive WS status from subscription events presence
  // (urql subscription status isn't directly exposed; we infer from events)
  const wsStatus: 'connecting' | 'connected' | 'error' =
    events.length > 0 ? 'connected' : 'connecting';

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100">
      <title>OBS Pipeline Dashboard</title>
      <Header services={services} wsStatus={wsStatus} />
      <AlertStrip alerts={alerts} />
      <main className="flex flex-col gap-4 p-6">
        <StatCards
          totalEvents={metrics.totalEvents}
          errorRate={metrics.errorRate}
          activeServices={metrics.activeServices}
          isPending={statsPending}
        />
        <EventsChart stats={stats} />
        <div className="grid grid-cols-2 gap-4">
          <LiveEventFeed events={events} />
          <AlertsTable alerts={alerts} />
        </div>
      </main>
    </div>
  );
}

export default function App() {
  const [filter, setFilter] = useState<FilterState>({
    service: null,
    timeRange: '1h',
  });

  return (
    <Provider value={urqlClient}>
      <FilterContext value={{ filter, setFilter }}>
        <Dashboard />
      </FilterContext>
    </Provider>
  );
}
