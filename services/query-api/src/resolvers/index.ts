import { GraphQLScalarType, Kind, type ValueNode } from 'graphql';
import type { IEventService } from '../services/event.service';
import type { IStatsService } from '../services/stats.service';
import type { IAlertService } from '../services/alert.service';
import { buildEventResolvers } from './event.resolver';
import { buildStatsResolvers } from './stats.resolver';
import { buildAlertResolvers } from './alert.resolver';

// Serialize any JSON-compatible value as-is
const JSONScalar = new GraphQLScalarType({
  name: 'JSON',
  description: 'Arbitrary JSON value',
  serialize: value => value,
  parseValue: value => value,
  parseLiteral(ast: ValueNode): unknown {
    if (ast.kind === Kind.STRING) return ast.value;
    if (ast.kind === Kind.BOOLEAN) return ast.value;
    if (ast.kind === Kind.INT || ast.kind === Kind.FLOAT) return parseFloat(ast.value);
    if (ast.kind === Kind.NULL) return null;
    if (ast.kind === Kind.LIST) return ast.values.map(v => JSONScalar.parseLiteral(v, {}));
    if (ast.kind === Kind.OBJECT) {
      return ast.fields.reduce<Record<string, unknown>>((acc, field) => {
        acc[field.name.value] = JSONScalar.parseLiteral(field.value, {});
        return acc;
      }, {});
    }
    return null;
  },
});

export function buildResolvers(
  eventService: IEventService,
  statsService: IStatsService,
  alertService: IAlertService,
) {
  return {
    JSON: JSONScalar,
    Query: {
      ...buildEventResolvers(eventService).Query,
      ...buildStatsResolvers(statsService).Query,
      ...buildAlertResolvers(alertService).Query,
    },
  };
}