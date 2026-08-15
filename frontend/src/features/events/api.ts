import { apiRequest } from '../../shared/api/client';
import type { EventCatalog } from './types';

export function getUpcomingEvents() {
  return apiRequest<EventCatalog>('/feed/events');
}
