import type { Paper } from './types';

const mockPapers: Paper[] = [
  {
    id: 'self-refining-language-models',
    title: 'Self-Refining Language Models via Online Critique',
    authors: 'Park, Iyer, Zhang +3',
    description:
      'A training-time framework where a model generates, critiques, and revises its own responses through an internal reward signal, improving reasoning accuracy on math and code benchmarks without additional human labels.',
    publishedAt: '2026-06-12T00:00:00.000Z',
    popularityScore: 942,
    wantToRead: false,
    githubUrl: 'https://github.com',
    githubStars: 213,
  },
  {
    id: 'moe-lite',
    title: 'MoE-Lite: Sparse Mixture-of-Experts for On-Device Inference',
    authors: 'Nakamura, Ortega',
    description:
      'Introduces a lightweight expert-routing scheme that runs 8-expert MoE models on consumer hardware, cutting active parameters by 4× while preserving 97% of dense-model quality across instruction-following tasks.',
    publishedAt: '2026-06-13T00:00:00.000Z',
    popularityScore: 671,
    wantToRead: true,
    githubUrl: 'https://github.com',
    githubStars: 156,
  },
  {
    id: 'rag-bench',
    title: 'RAG-Bench: A Unified Benchmark for Retrieval-Augmented Generation',
    authors: 'Fernandez, Cho, Williams +2',
    description:
      'Proposes a standardized evaluation suite covering retrieval quality, faithfulness, and answer relevance across 12 domains, revealing that current RAG pipelines overfit to surface-level lexical overlap.',
    publishedAt: '2026-06-10T00:00:00.000Z',
    popularityScore: 1240,
    wantToRead: false,
  },
  {
    id: 'chain-of-agents',
    title: 'Chain-of-Agents: Collaborative Reasoning Across Specialized LLMs',
    authors: 'Dubois, Ananthaswamy',
    description:
      'A multi-agent orchestration method where specialist models hand off intermediate state through a shared scratchpad, achieving stronger long-horizon planning than monolithic models on agentic web tasks.',
    publishedAt: '2026-06-06T00:00:00.000Z',
    popularityScore: 805,
    wantToRead: false,
    githubUrl: 'https://github.com',
    githubStars: 489,
  },
  {
    id: 'visual-grounding',
    title: 'Visual Grounding in Multimodal Foundation Models at Scale',
    authors: 'Petrova, Lindqvist +4',
    description:
      'Studies how vision-language models localize referring expressions as parameter count grows, finding emergent fine-grained grounding above 30B params and releasing a 2M-image evaluation set.',
    publishedAt: '2026-05-30T00:00:00.000Z',
    popularityScore: 388,
    wantToRead: false,
    githubUrl: 'https://github.com',
    githubStars: 74,
  },
];

const delay = (ms: number) =>
  new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });

export async function fetchTrendingPapers(): Promise<Paper[]> {
  await delay(900);
  return mockPapers;
}
