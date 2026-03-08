import { createClient, cacheExchange, fetchExchange, subscriptionExchange } from 'urql';
import { createClient as createWSClient } from 'graphql-ws';

const isBrowser = typeof window !== 'undefined';

const WS_URL = isBrowser
  ? `${window.location.protocol === 'https:' ? 'wss' : 'ws'}://${window.location.host}/graphql`
  : 'ws://localhost:4000/graphql';

const wsClient = isBrowser
  ? createWSClient({
      url: WS_URL,
      retryAttempts: 5,
    })
  : null;

export const urqlClient = createClient({
  url: '/graphql',
  exchanges: [
    cacheExchange,
    fetchExchange,

    ...(wsClient
      ? [
          subscriptionExchange({
            // urql v5: request.query is already a string (not a DocumentNode).
            forwardSubscription(request) {
              return {
                subscribe(sink) {
                  const unsubscribe = wsClient.subscribe(
                    {
                      query: request.query ?? '',
                      variables: request.variables,
                    },
                    sink,
                  );
                  return { unsubscribe };
                },
              };
            },
          }),
        ]
      : []),
  ],
});
