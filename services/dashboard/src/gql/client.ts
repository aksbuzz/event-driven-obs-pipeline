import { createClient, cacheExchange, fetchExchange, subscriptionExchange } from 'urql';
import { createClient as createWSClient } from 'graphql-ws';
import { print } from 'graphql';

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
            forwardSubscription(operation) {
              return {
                subscribe(sink) {
                  const unsubscribe = wsClient.subscribe(
                    {
                      query: print(operation.query as any),
                      variables: operation.variables,
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
