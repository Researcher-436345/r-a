import type { Locale } from '../../shared/i18n/i18n-context';
import type { ResearchMode } from './types';

const ruInitialResponse = `## Наиболее производительные подходы к Web Search / Deep Research (2025–2026)

Поле развивается крайне быстро — за последний год вышло несколько ключевых подходов, которые можно разделить на две большие парадигмы: **тренировочные** (научить модель быть исследователем) и **инференс-оркестрационные** (построить инфраструктуру вокруг любой модели). Вот как выглядит текущий ландшафт.

---

### Тренировочные подходы (основной тренд)

#### 1. QUEST — самый сбалансированный открытый агент

**QUEST** (OSU + Amazon, май 2026) — семейство моделей 2B–35B, лучший open-weight агент общего назначения. Ключевая идея: **рубриковые деревья** (rubric trees) — иерархическое разложение критериев ответа, которые позволяют синтезировать тренировочные данные с верифицируемыми наградами без человеческой разметки.

Трёхстадийный пайплайн: **Mid-Training → SFT → RL**. Результаты QUEST-35B на восьми бенчмарках:

| Бенчмарк | QUEST-35B | OpenAI Deep Research | GPT-5 |
| --- | --- | --- | --- |
| BrowseComp | **64.6%** *(с discard-all)* | 51.5% | 59.9% |
| GAIA (text) | **80.8%** | 67.4% | 76.4% |
| DeepResearch Bench | **48.2%** | 47.0% | — |

#### 2. SFR-DR — компактный специалист

**SFR-DeepResearch** (Salesforce) — 20B-модель, обученная только на agentic-траекториях глубокого поиска. Уступает QUEST в универсальности, но при 20B параметрах достигает 82% качества фронтир-систем на HLE-подмножестве, что делает её лучшим выбором для self-hosted сценариев.`;

const enInitialResponse = `## Highest-performing approaches to Web Search / Deep Research (2025–2026)

The field is moving extremely quickly. Over the past year, several important approaches have emerged, and they fall into two broad paradigms: **training-based methods** (teaching a model to act as a researcher) and **inference orchestration** (building research infrastructure around any capable model). Here is the current landscape.

---

### Training-based approaches (the main trend)

#### 1. QUEST — the most balanced open research agent

**QUEST** (OSU + Amazon, May 2026) is a family of 2B–35B models and the strongest general-purpose open-weight agent in this comparison. Its key idea is **rubric trees**: a hierarchical decomposition of answer criteria that makes it possible to synthesize training data with verifiable rewards and no human labeling.

The training pipeline has three stages: **Mid-Training → SFT → RL**. QUEST-35B reports the following results:

| Benchmark | QUEST-35B | OpenAI Deep Research | GPT-5 |
| --- | --- | --- | --- |
| BrowseComp | **64.6%** *(with discard-all)* | 51.5% | 59.9% |
| GAIA (text) | **80.8%** | 67.4% | 76.4% |
| DeepResearch Bench | **48.2%** | 47.0% | — |

#### 2. SFR-DR — a compact specialist

**SFR-DeepResearch** (Salesforce) is a 20B model trained exclusively on agentic deep-search trajectories. It is less general than QUEST, but at 20B parameters it reaches 82% of frontier-system quality on the HLE subset, making it the strongest option for self-hosted scenarios.`;

const initialResponses: Record<Locale, Record<ResearchMode, string>> = {
  ru: {
    web: ruInitialResponse,
    deep: ruInitialResponse,
  },
  en: {
    web: enInitialResponse,
    deep: enInitialResponse,
  },
};

const followUpResponses: Record<Locale, Record<ResearchMode, string>> = {
  ru: {
    web: `### Как уточнить результат

Для этого вопроса я бы сделал ещё один короткий поисковый проход:

1. превратил критерии из вопроса в отдельные запросы;
2. оставил только первичные или официальные источники;
3. свёл найденное в таблицу **подход / доказательство / ограничение**.

> В mock-режиме новые источники не загружаются, поэтому это демонстрация продолжения диалога.`,
    deep: `### Следующая исследовательская ветка

Я бы добавил этот вопрос в план как отдельную ветку и проверил её в три шага:

- сформулировать проверяемую гипотезу;
- собрать минимум два независимых первичных источника;
- записать не только подтверждения, но и контрпример.

После этого вывод можно пометить как **подтверждённый**, **предварительный** или **неопределённый**.`,
  },
  en: {
    web: `### How to refine the result

I would run one more focused search pass for this question:

1. turn each criterion into a separate query;
2. retain only primary or first-party sources;
3. compare findings in an **approach / evidence / limitation** table.

> Mock mode does not retrieve new sources, so this is a demonstration of a follow-up response.`,
    deep: `### The next research branch

I would add this question as a separate branch and test it in three steps:

- express it as a falsifiable hypothesis;
- collect at least two independent primary sources;
- record counterexamples as well as supporting evidence.

The resulting claim can then be marked as **supported**, **provisional**, or **uncertain**.`,
  },
};

export function getMockResponse(locale: Locale, mode: ResearchMode) {
  return initialResponses[locale][mode];
}

export function getMockFollowUpResponse(locale: Locale, mode: ResearchMode) {
  return followUpResponses[locale][mode];
}
