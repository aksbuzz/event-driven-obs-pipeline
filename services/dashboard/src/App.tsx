import { Component, ErrorInfo, ReactNode, useState } from 'react';
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

class ErrorBoundary extends Component<{ children: ReactNode }, { error: Error | null }> {
  constructor(props: { children: ReactNode }) {
    super(props);
    this.state = { error: null };
  }
  static getDerivedStateFromError(error: Error) {
    return { error };
  }
  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error('Dashboard error:', error, info.componentStack);
  }
  render() {
    if (this.state.error) {
      return (
        <div className="min-h-screen bg-gray-950 text-gray-100 flex items-center justify-center">
          <div className="text-center">
            <p className="text-red-400 text-lg font-semibold">Dashboard error</p>
            <p className="text-gray-400 mt-2 text-sm">{this.state.error.message}</p>
            <button
              className="mt-4 px-4 py-2 bg-gray-800 rounded text-sm hover:bg-gray-700"
              onClick={() => this.setState({ error: null })}
            >
              Retry
            </button>
          </div>
        </div>
      );
    }
    return this.props.children;
  }
}

function Dashboard() {
  const { data: stats, isPending: statsPending } = useStats();
  const { data: alerts } = useAlerts();
  const { events, wsStatus } = useLiveEvents();

  const safeStats = stats ?? [];
  const metrics = deriveMetrics(safeStats);

  // Accumulate services so the dropdown doesn't shrink when a service filter is active.
  const [allServices, setAllServices] = useState<string[]>([]);
  const discovered = [...new Set(safeStats.map(s => s.service))].sort();
  if (discovered.some(s => !allServices.includes(s))) {
    setAllServices(prev => [...new Set([...prev, ...discovered])].sort());
  }

  return (
    <div className="min-h-screen bg-gray-950 text-gray-100">
      <title>OBS Pipeline Dashboard</title>
      <Header services={allServices} wsStatus={wsStatus} />
      <AlertStrip alerts={alerts} />
      <main className="flex flex-col gap-4 p-6">
        <StatCards
          totalEvents={metrics.totalEvents}
          errorRate={metrics.errorRate}
          activeServices={metrics.activeServices}
          isPending={statsPending}
        />
        <EventsChart stats={safeStats} />
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
        <ErrorBoundary>
          <Dashboard />
        </ErrorBoundary>
      </FilterContext>
    </Provider>
  );
}
