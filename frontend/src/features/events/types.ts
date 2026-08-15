export interface TechEvent {
  id: string;
  title: string;
  summary: string;
  start_date: string;
  end_date: string;
  city: string;
  country: string;
  format: 'in_person' | 'online' | 'hybrid';
  kind: 'conference' | 'meetup';
  region: 'ru' | 'global';
  topics: string[];
  url: string;
  registration_url?: string;
  source_url: string;
  featured: boolean;
}

export interface EventCatalog {
  items: TechEvent[];
  updated_at: string;
  next_update_at: string;
  automatic: boolean;
}
