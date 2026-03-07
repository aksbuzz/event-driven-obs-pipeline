import { IResolvers } from 'mercurius';
import { activeSubscriptions } from '../metrics';

export const subscriptionResolvers: IResolvers = {
  Subscription: {
    eventReceived: {
      subscribe: async (root, args: { service?: string }, { pubsub }) => {
        activeSubscriptions.inc();

        const topic = args?.service ? `EVENT_RECEIVED:${args.service}` : 'EVENT_RECEIVED';
        const iterator = await pubsub.subscribe(topic);
        return trackIterator(iterator);
      },
    },
  },
};

// Decrement the gauge when the subscription closes
function trackIterator(iterator: AsyncIterator<any>) {
  const origReturn = iterator.return?.bind(iterator);

  iterator.return = async (...args) => {
    activeSubscriptions.dec();
    return origReturn ? origReturn(...args) : { done: true, value: undefined };
  };

  return iterator;
}