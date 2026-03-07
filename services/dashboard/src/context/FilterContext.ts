import { createContext } from 'react';
import { FilterContextValue } from '../types';

export const FilterContext = createContext<FilterContextValue>({
  filter: { service: null, timeRange: '1h' },
  setFilter: () => {},
});
