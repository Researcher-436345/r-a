import { useQuery } from '@tanstack/react-query';
import {
  CalendarDays,
  Check,
  ChevronLeft,
  ChevronRight,
  Download,
  ExternalLink,
  Globe2,
  MapPin,
  MonitorUp,
  RefreshCw,
  Search,
  Sparkles,
} from 'lucide-react';
import { useMemo, useState } from 'react';

import { getUpcomingEvents } from '../../features/events/api';
import type { TechEvent } from '../../features/events/types';
import { useI18n } from '../../shared/i18n/i18n-context';

type RegionFilter = 'all' | 'ru' | 'global';
type TopicFilter = 'all' | 'ai' | 'engineering';

const copy = {
  ru: {
    eyebrow: 'AI · ML · ENGINEERING',
    title: 'События',
    subtitle: 'Конференции и митапы, за которыми действительно стоит следить.',
    search: 'Найти событие или тему',
    all: 'Все',
    russia: 'Россия',
    world: 'Весь мир',
    ai: 'AI & ML',
    engineering: 'Разработка',
    next: 'Ближайшее важное событие',
    official: 'Официальный сайт',
    calendar: 'В календарь',
    schedule: 'Календарь',
    clearDate: 'Показать все даты',
    auto: 'Обновляется каждый день',
    autoHint: 'Новые даты проверяются по официальным сайтам организаторов.',
    empty: 'По выбранным фильтрам событий пока нет.',
    error: 'Не удалось загрузить события',
    retry: 'Попробовать снова',
    loading: 'Собираем ближайшие события…',
    online: 'Онлайн',
    hybrid: 'Онлайн и офлайн',
    inPerson: 'Очно',
    conference: 'Конференция',
    meetup: 'Митап',
    today: 'Сегодня',
    tomorrow: 'Завтра',
    days: 'дн.',
  },
  en: {
    eyebrow: 'AI · ML · ENGINEERING',
    title: 'Events',
    subtitle: 'Conferences and meetups genuinely worth keeping on your radar.',
    search: 'Find an event or topic',
    all: 'All',
    russia: 'Russia',
    world: 'Worldwide',
    ai: 'AI & ML',
    engineering: 'Engineering',
    next: 'Next important event',
    official: 'Official website',
    calendar: 'Add to calendar',
    schedule: 'Calendar',
    clearDate: 'Show all dates',
    auto: 'Updated every day',
    autoHint: 'New dates are verified against official organizer websites.',
    empty: 'No events match these filters yet.',
    error: 'Could not load events',
    retry: 'Try again',
    loading: 'Collecting upcoming events…',
    online: 'Online',
    hybrid: 'Online and in person',
    inPerson: 'In person',
    conference: 'Conference',
    meetup: 'Meetup',
    today: 'Today',
    tomorrow: 'Tomorrow',
    days: 'days',
  },
} as const;

const aiTopics = ['ai', 'ml', 'llm', 'nlp', 'vision', 'deep learning', 'rec', 'agent'];

function parseDate(value: string) {
  return new Date(`${value}T12:00:00`);
}

function toISODate(value: Date) {
  const year = value.getFullYear();
  const month = String(value.getMonth() + 1).padStart(2, '0');
  const day = String(value.getDate()).padStart(2, '0');
  return `${year}-${month}-${day}`;
}

function monthKey(value: Date) {
  return `${value.getFullYear()}-${String(value.getMonth() + 1).padStart(2, '0')}`;
}

function addDays(value: Date, amount: number) {
  const next = new Date(value);
  next.setDate(next.getDate() + amount);
  return next;
}

function dateRangeIncludes(event: TechEvent, date: string) {
  return event.start_date <= date && event.end_date >= date;
}

function eventMatchesTopic(event: TechEvent, filter: TopicFilter) {
  if (filter === 'all') return true;
  const haystack = `${event.title} ${event.summary} ${event.topics.join(' ')}`.toLowerCase();
  const isAI = aiTopics.some((topic) => haystack.includes(topic));
  return filter === 'ai' ? isAI : !isAI || /backend|cloud|security|devops|architecture|engineering|highload/.test(haystack);
}

function escapeICS(value: string) {
  return value.replace(/\\/g, '\\\\').replace(/\n/g, '\\n').replace(/,/g, '\\,').replace(/;/g, '\\;');
}

function downloadICS(event: TechEvent) {
  const start = event.start_date.split('-').join('');
  const endExclusive = toISODate(addDays(parseDate(event.end_date), 1)).split('-').join('');
  const location = [event.city, event.country].filter(Boolean).join(', ');
  const body = [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//r-a//Tech Events//RU',
    'CALSCALE:GREGORIAN',
    'BEGIN:VEVENT',
    `UID:${event.id}@r-a`,
    `DTSTART;VALUE=DATE:${start}`,
    `DTEND;VALUE=DATE:${endExclusive}`,
    `SUMMARY:${escapeICS(event.title)}`,
    `DESCRIPTION:${escapeICS(`${event.summary}\n${event.url}`)}`,
    `LOCATION:${escapeICS(location)}`,
    `URL:${event.url}`,
    'END:VEVENT',
    'END:VCALENDAR',
  ].join('\r\n');
  const link = document.createElement('a');
  link.href = URL.createObjectURL(new Blob([body], { type: 'text/calendar;charset=utf-8' }));
  link.download = `${event.id}.ics`;
  link.click();
  URL.revokeObjectURL(link.href);
}

export function EventsPage() {
  const { locale } = useI18n();
  const c = copy[locale];
  const [region, setRegion] = useState<RegionFilter>('all');
  const [topic, setTopic] = useState<TopicFilter>('all');
  const [query, setQuery] = useState('');
  const [selectedDate, setSelectedDate] = useState<string | null>(null);
  const [visibleMonth, setVisibleMonth] = useState(() => {
    const now = new Date();
    return new Date(now.getFullYear(), now.getMonth(), 1);
  });

  const eventsQuery = useQuery({
    queryKey: ['events', 'upcoming'],
    queryFn: getUpcomingEvents,
    staleTime: 15 * 60 * 1000,
  });
  const events = eventsQuery.data?.items ?? [];

  const filteredEvents = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return events.filter((event) => {
      if (region !== 'all' && event.region !== region) return false;
      if (!eventMatchesTopic(event, topic)) return false;
      if (selectedDate && !dateRangeIncludes(event, selectedDate)) return false;
      if (!needle) return true;
      return `${event.title} ${event.summary} ${event.city} ${event.country} ${event.topics.join(' ')}`
        .toLowerCase()
        .includes(needle);
    });
  }, [events, query, region, selectedDate, topic]);

  const groupedEvents = useMemo(() => {
    const groups = new Map<string, TechEvent[]>();
    filteredEvents.forEach((event) => {
      const key = monthKey(parseDate(event.start_date));
      groups.set(key, [...(groups.get(key) ?? []), event]);
    });
    return [...groups.entries()];
  }, [filteredEvents]);

  const nextEvent = events.find((event) => event.featured) ?? events[0];

  return (
    <div className="events-page">
      <header className="events-hero">
        <div>
          <p className="events-hero__eyebrow"><Sparkles size={14} /> {c.eyebrow}</p>
          <h1>{c.title}</h1>
          <p className="events-hero__subtitle">{c.subtitle}</p>
        </div>
        <div className="events-refresh-note" title={c.autoHint}>
          <span className="events-refresh-note__icon"><RefreshCw size={15} /></span>
          <span><strong>{c.auto}</strong><small>{c.autoHint}</small></span>
        </div>
      </header>

      {nextEvent ? <FeaturedEvent event={nextEvent} locale={locale} labels={c} /> : null}

      <div className="events-toolbar" aria-label="Event filters">
        <label className="events-search">
          <Search size={17} aria-hidden="true" />
          <span className="sr-only">{c.search}</span>
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={c.search} />
        </label>
        <div className="events-filter-group">
          <FilterButton active={topic === 'all'} onClick={() => setTopic('all')}>{c.all}</FilterButton>
          <FilterButton active={topic === 'ai'} onClick={() => setTopic('ai')}>{c.ai}</FilterButton>
          <FilterButton active={topic === 'engineering'} onClick={() => setTopic('engineering')}>{c.engineering}</FilterButton>
        </div>
        <div className="events-filter-group">
          <FilterButton active={region === 'ru'} onClick={() => setRegion(region === 'ru' ? 'all' : 'ru')}>{c.russia}</FilterButton>
          <FilterButton active={region === 'global'} onClick={() => setRegion(region === 'global' ? 'all' : 'global')}>{c.world}</FilterButton>
        </div>
      </div>

      <div className="events-layout">
        <section className="events-list" aria-live="polite">
          {eventsQuery.isPending ? <EventsLoading label={c.loading} /> : null}
          {eventsQuery.isError ? (
            <div className="events-state events-state--error">
              <p>{c.error}</p>
              <button type="button" onClick={() => void eventsQuery.refetch()}>{c.retry}</button>
            </div>
          ) : null}
          {!eventsQuery.isPending && !eventsQuery.isError && groupedEvents.length === 0 ? (
            <div className="events-state"><CalendarDays size={24} /><p>{c.empty}</p></div>
          ) : null}
          {groupedEvents.map(([month, monthEvents]) => (
            <div className="events-month" key={month}>
              <div className="events-month__heading">
                <span>{formatMonth(month, locale)}</span><span>{monthEvents.length}</span>
              </div>
              <div className="events-month__cards">
                {monthEvents.map((event) => <EventCard key={event.id} event={event} locale={locale} labels={c} />)}
              </div>
            </div>
          ))}
        </section>

        <aside className="events-calendar-panel">
          <MiniCalendar
            events={events}
            locale={locale}
            month={visibleMonth}
            selectedDate={selectedDate}
            title={c.schedule}
            clearLabel={c.clearDate}
            onMonthChange={setVisibleMonth}
            onDateSelect={setSelectedDate}
          />
        </aside>
      </div>
    </div>
  );
}

function FilterButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: string }) {
  return <button type="button" className={active ? 'events-filter events-filter--active' : 'events-filter'} onClick={onClick}>{active ? <Check size={13} /> : null}{children}</button>;
}

function FeaturedEvent({ event, locale, labels }: { event: TechEvent; locale: 'ru' | 'en'; labels: typeof copy.ru | typeof copy.en }) {
  const days = Math.max(0, Math.ceil((parseDate(event.start_date).getTime() - Date.now()) / 86_400_000));
  const countdown = days === 0 ? labels.today : days === 1 ? labels.tomorrow : `${days} ${labels.days}`;
  return (
    <article className="events-featured">
      <div className="events-featured__date">
        <span>{countdown}</span>
        <strong>{formatDateRange(event, locale)}</strong>
      </div>
      <div className="events-featured__body">
        <p>{labels.next}</p>
        <h2>{event.title}</h2>
        <span><MapPin size={15} /> {event.city}, {event.country}</span>
      </div>
      <a href={event.url} target="_blank" rel="noreferrer" className="events-featured__link">
        {labels.official}<ExternalLink size={15} />
      </a>
    </article>
  );
}

function EventCard({ event, locale, labels }: { event: TechEvent; locale: 'ru' | 'en'; labels: typeof copy.ru | typeof copy.en }) {
  const date = parseDate(event.start_date);
  const formatLabel = event.format === 'online' ? labels.online : event.format === 'hybrid' ? labels.hybrid : labels.inPerson;
  return (
    <article className={event.featured ? 'event-card event-card--featured' : 'event-card'}>
      <div className="event-card__date" aria-label={formatDateRange(event, locale)}>
        <span>{new Intl.DateTimeFormat(locale, { month: 'short' }).format(date).replace('.', '')}</span>
        <strong>{date.getDate()}</strong>
        {event.end_date !== event.start_date ? <small>— {parseDate(event.end_date).getDate()}</small> : null}
      </div>
      <div className="event-card__content">
        <div className="event-card__topline">
          <span className={`event-card__region event-card__region--${event.region}`}><Globe2 size={12} />{event.region === 'ru' ? labels.russia : labels.world}</span>
          <span>{event.kind === 'meetup' ? labels.meetup : labels.conference}</span>
        </div>
        <h3><a href={event.url} target="_blank" rel="noreferrer">{event.title}</a></h3>
        <p>{event.summary}</p>
        <div className="event-card__meta">
          <span><MapPin size={14} />{event.city}{event.country ? `, ${event.country}` : ''}</span>
          <span><MonitorUp size={14} />{formatLabel}</span>
        </div>
        <div className="event-card__topics">{event.topics.slice(0, 4).map((item) => <span key={item}>{item}</span>)}</div>
      </div>
      <div className="event-card__actions">
        <a href={event.registration_url ?? event.url} target="_blank" rel="noreferrer">{labels.official}<ExternalLink size={14} /></a>
        <button type="button" onClick={() => downloadICS(event)}><Download size={14} />{labels.calendar}</button>
      </div>
    </article>
  );
}

function MiniCalendar({ events, locale, month, selectedDate, title, clearLabel, onMonthChange, onDateSelect }: {
  events: TechEvent[];
  locale: 'ru' | 'en';
  month: Date;
  selectedDate: string | null;
  title: string;
  clearLabel: string;
  onMonthChange: (value: Date) => void;
  onDateSelect: (value: string | null) => void;
}) {
  const first = new Date(month.getFullYear(), month.getMonth(), 1);
  const offset = (first.getDay() + 6) % 7;
  const gridStart = addDays(first, -offset);
  const days = Array.from({ length: 42 }, (_, index) => addDays(gridStart, index));
  const weekdays = Array.from({ length: 7 }, (_, index) => new Intl.DateTimeFormat(locale, { weekday: 'short' }).format(addDays(new Date(2024, 0, 1), index)).slice(0, 2));
  return (
    <div className="mini-calendar">
      <div className="mini-calendar__title"><span><CalendarDays size={16} />{title}</span></div>
      <div className="mini-calendar__month">
        <button type="button" aria-label="Previous month" onClick={() => onMonthChange(new Date(month.getFullYear(), month.getMonth() - 1, 1))}><ChevronLeft size={17} /></button>
        <strong>{new Intl.DateTimeFormat(locale, { month: 'long', year: 'numeric' }).format(month)}</strong>
        <button type="button" aria-label="Next month" onClick={() => onMonthChange(new Date(month.getFullYear(), month.getMonth() + 1, 1))}><ChevronRight size={17} /></button>
      </div>
      <div className="mini-calendar__weekdays">{weekdays.map((day, index) => <span key={`${day}-${index}`}>{day}</span>)}</div>
      <div className="mini-calendar__grid">
        {days.map((day) => {
          const iso = toISODate(day);
          const matching = events.filter((event) => dateRangeIncludes(event, iso));
          const outside = day.getMonth() !== month.getMonth();
          const isToday = iso === toISODate(new Date());
          const classNames = ['mini-calendar__day', outside ? 'mini-calendar__day--outside' : '', matching.length ? 'mini-calendar__day--event' : '', selectedDate === iso ? 'mini-calendar__day--selected' : '', isToday ? 'mini-calendar__day--today' : ''].filter(Boolean).join(' ');
          return <button type="button" className={classNames} key={iso} title={matching.map((event) => event.title).join(', ')} onClick={() => onDateSelect(selectedDate === iso ? null : iso)}><span>{day.getDate()}</span>{matching.length ? <i /> : null}</button>;
        })}
      </div>
      {selectedDate ? <button type="button" className="mini-calendar__clear" onClick={() => onDateSelect(null)}>{clearLabel}</button> : null}
      <div className="mini-calendar__legend"><i /><span>{events.filter((event) => monthKey(parseDate(event.start_date)) === monthKey(month)).length} {locale === 'ru' ? 'событий в этом месяце' : 'events this month'}</span></div>
    </div>
  );
}

function formatDateRange(event: TechEvent, locale: 'ru' | 'en') {
  const start = parseDate(event.start_date);
  const end = parseDate(event.end_date);
  const formatter = new Intl.DateTimeFormat(locale, { day: 'numeric', month: 'long', year: 'numeric' });
  if (event.start_date === event.end_date) return formatter.format(start);
  if (start.getMonth() === end.getMonth()) {
    return `${start.getDate()}–${formatter.format(end)}`;
  }
  return `${new Intl.DateTimeFormat(locale, { day: 'numeric', month: 'long' }).format(start)} — ${formatter.format(end)}`;
}

function formatMonth(value: string, locale: 'ru' | 'en') {
  const [year, month] = value.split('-').map(Number);
  const label = new Intl.DateTimeFormat(locale, { month: 'long', year: 'numeric' }).format(new Date(year, month - 1, 1));
  return label.charAt(0).toUpperCase() + label.slice(1);
}

function EventsLoading({ label }: { label: string }) {
  return <div className="events-state"><RefreshCw className="events-state__spinner" size={22} /><p>{label}</p></div>;
}
