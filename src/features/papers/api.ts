import type { Paper } from './types';

const mockPapers: Paper[] = [
  {
    id: 'agentic-literature-review',
    title: 'Agentic Literature Review with Verifiable Citation Graphs',
    description:
      'Introduces a research assistant pipeline that decomposes broad scientific questions, builds citation-aware evidence graphs, and validates claims against source passages.',
    publishedAt: '2026-06-10T09:30:00.000Z',
    popularityScore: 94,
    wantToRead: true,
    githubUrl: 'https://github.com/example/agentic-literature-review',
  },
  {
    id: 'long-context-rag-evaluation',
    title: 'Measuring Long-Context Retrieval for Scientific Question Answering',
    description:
      'A benchmark for comparing long-context LLMs and retrieval-augmented systems on multi-paper synthesis tasks with delayed evidence requirements.',
    publishedAt: '2026-06-07T14:10:00.000Z',
    popularityScore: 88,
    wantToRead: false,
    githubUrl: 'https://github.com/example/long-context-rag-eval',
  },
  {
    id: 'multimodal-paper-agents',
    title: 'Multimodal Paper Agents for Figures, Tables, and Equations',
    description:
      'Shows how vision-language models can inspect PDF figures, parse table structure, and connect visual evidence to textual claims during paper discovery.',
    publishedAt: '2026-06-02T17:45:00.000Z',
    popularityScore: 82,
    wantToRead: false,
  },
  {
    id: 'preference-learning-research-feeds',
    title: 'Preference Learning for Personalized Research Feeds',
    description:
      'Models researcher intent from saves, skips, project notes, and follow-up questions to rank emerging papers without trapping users in narrow topics.',
    publishedAt: '2026-05-29T11:00:00.000Z',
    popularityScore: 76,
    wantToRead: true,
    githubUrl: 'https://github.com/example/research-feed-preferences',
  },
  {
    id: 'reasoning-trace-compression',
    title: 'Trace Compression Improves Deliberative Reasoning Benchmarks',
    description:
      'Studies compact intermediate reasoning traces for LLM agents, reducing token cost while preserving performance on scientific planning and critique tasks.',
    publishedAt: '2026-05-25T08:20:00.000Z',
    popularityScore: 71,
    wantToRead: false,
  },
];

const delay = (ms: number) =>
  new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });

export async function fetchTrendingPapers(): Promise<Paper[]> {
  await delay(650);
  return mockPapers;
}
