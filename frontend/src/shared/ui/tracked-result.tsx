import type { ReactNode } from 'react';
import type { ResearchFeature } from '../lib/analytics';
import { useResultAnalytics } from '../lib/use-result-analytics';

export function TrackedResult({ feature, resultKey, paperId, resultId, className, children }: {
  feature: ResearchFeature; resultKey?: string; paperId?: string; resultId?: string;
  className?: string; children: ReactNode;
}) {
  const ref = useResultAnalytics(feature, resultKey, paperId, resultId);
  return <div ref={ref} className={className} data-chat-message-id={resultId} data-research-result-id={resultId}>{children}</div>;
}
