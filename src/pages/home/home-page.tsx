import { AskAiBox } from '../../features/chat/components/ask-ai-box';
import { TrendingPapers } from '../../features/papers/components/trending-papers';

export function HomePage() {
  return (
    <div className="home-page">
      <AskAiBox />
      <TrendingPapers />
    </div>
  );
}
