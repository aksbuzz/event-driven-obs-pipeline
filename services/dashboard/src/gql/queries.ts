import { gql } from 'urql';

export const STATS_QUERY = gql`
  query Stats($from: String!, $to: String!, $service: String) {
    stats(filter: { from: $from, to: $to, service: $service }) {
      bucket
      service
      level
      eventCount
      avgDurationMs
      maxDurationMs
    }
  }
`;

export const ALERTS_QUERY = gql`
  query Alerts($from: String!, $service: String) {
    alerts(filter: { from: $from, service: $service, limit: 20 }) {
      alertId
      service
      rule
      severity
      firedAt
      details
    }
  }
`;

export const EVENT_SUBSCRIPTION = gql`
  subscription LiveEvents($service: String) {
    eventReceived(service: $service) {
      eventId
      service
      environment
      level
      timestamp
      enrichment
    }
  }
`;
