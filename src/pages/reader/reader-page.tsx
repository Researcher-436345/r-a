import { ReaderChatPanel } from '../../features/reader/components/reader-chat-panel';
import { ReaderPdfViewer } from '../../features/reader/components/reader-pdf-viewer';

export function ReaderPage() {
  return (
    <div className="reader-page">
      <ReaderPdfViewer />
      <ReaderChatPanel />
    </div>
  );
}
