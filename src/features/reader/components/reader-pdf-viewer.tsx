import {
  Bookmark,
  BookmarkCheck,
  Download,
  FileText,
  Minus,
  Plus,
} from 'lucide-react';
import { useState } from 'react';

import { useI18n } from '../../../shared/i18n/i18n-context';
import { readerPaper, readerStrings } from '../reader-data';

export function ReaderPdfViewer() {
  const { locale } = useI18n();
  const [isBookmarked, setIsBookmarked] = useState(false);
  const text = readerStrings[locale];
  const BookmarkIcon = isBookmarked ? BookmarkCheck : Bookmark;

  return (
    <section className="reader-viewer" aria-label="PDF viewer">
      <div className="reader-toolbar">
        <button
          className={
            isBookmarked
              ? 'reader-bookmark-button reader-bookmark-button--active'
              : 'reader-bookmark-button'
          }
          type="button"
          title={isBookmarked ? text.bookmarkRemove : text.bookmarkAdd}
          aria-label={isBookmarked ? text.bookmarkRemove : text.bookmarkAdd}
          onClick={() => setIsBookmarked((value) => !value)}
        >
          <BookmarkIcon aria-hidden="true" size={19} strokeWidth={2} />
        </button>

        <div className="reader-toolbar__divider" />

        <div className="reader-toolbar__paper">
          <div className="reader-toolbar__title">{readerPaper.title}</div>
          <div className="reader-toolbar__meta">{readerPaper.meta}</div>
        </div>

        <div className="reader-zoom" aria-label="Zoom controls">
          <button className="reader-zoom__button" type="button" title={text.zoomOut}>
            <Minus aria-hidden="true" size={16} strokeWidth={2} />
          </button>
          <span className="reader-zoom__value">110%</span>
          <button className="reader-zoom__button" type="button" title={text.zoomIn}>
            <Plus aria-hidden="true" size={16} strokeWidth={2} />
          </button>
        </div>

        <div className="reader-page-count">
          <FileText aria-hidden="true" size={15} strokeWidth={2} />
          <span>1 / 28</span>
        </div>

        <button className="reader-download-button" type="button" title={text.download}>
          <Download aria-hidden="true" size={17} strokeWidth={2} />
        </button>
      </div>

      <div className="reader-page-stack">
        <article className="reader-paper-page">
          <h1 className="reader-paper-title">{readerPaper.title}</h1>
          <div className="reader-paper-authors">{readerPaper.authors}</div>
          <div className="reader-paper-affiliation">{readerPaper.affiliation}</div>

          <div className="reader-figure">
            <span>figure 1 — QGF architecture diagram</span>
          </div>
          <p className="reader-figure-caption">
            <b>Figure 1:</b> We propose QGF (Q-Guided Flow), an RL algorithm that guides
            denoising steps of a policy trained via flow matching with critic gradient at
            test time. Our estimator avoids both taking gradient at noisy actions unseen
            during training and expensive, high-variance backpropagation through time.
          </p>

          <p className="reader-paper-paragraph">
            Expressive continuous control policies, such as diffusion and flow models, form
            the backbone of recent advances in scaling imitation learning for simulated and
            real robot control. While they are known to scale stably in the supervised
            imitation learning setting,{' '}
            <span className="pdf-hl">
              incorporating them into reinforcement learning (RL) pipelines for policy
              improvement has proven more difficult.
            </span>{' '}
            It often requires specialized training objectives or backpropagating through
            denoising processes, which cause well-known issues with stability and affect
            scalability.
          </p>

          <p className="reader-paper-paragraph reader-paper-paragraph--tight">
            To this end, we propose QGF (Q-Guided Flow), an RL algorithm that performs
            policy optimization entirely at test time. QGF works by pre-training both a
            reference flow policy (via a standard behavioral cloning objective) and a value
            function critic and, at test time, using the value gradient to guide the
            reference policy to generate higher-value actions without any additional policy
            learning.
          </p>

          <div className="reader-section-title">
            <span>1.</span>
            <span>Introduction</span>
          </div>
          <p className="reader-paper-paragraph">
            Reinforcement learning (RL) is a powerful framework for optimizing
            reward-maximizing behavior, but scaling RL algorithms—especially in the offline
            or off-policy setting—remains a significant challenge. A key difficulty lies in
            the complexity and instability of policy optimization: unlike simple supervised
            learning objectives that optimize towards a known fixed target, RL typically
            alternates between learning a value function and optimizing an actor to maximize
            this learned function.
          </p>
          <p className="reader-paper-paragraph reader-paper-paragraph--last">
            This challenge is particularly acute when using expressive policy classes such
            as diffusion and flow models. These models are attractive because they can
            represent complex, multimodal action distributions and have been shown to scale
            effectively when performing standard supervised learning via behavioral cloning.
          </p>
        </article>

        <article className="reader-paper-page reader-paper-page--second">
          <p className="reader-paper-paragraph">
            However, incorporating them into RL pipelines typically requires designing
            specialized training objectives or backpropagating through long denoising
            processes, which undermines their scalability and stability. This tension
            motivates an alternative approach: rather than designing a more scalable RL
            objective for policy optimization, can we train the actor with standard
            supervised learning and use test-time compute to optimize it against a value
            function?
          </p>
          <p className="reader-paper-paragraph reader-paper-paragraph--muted reader-paper-paragraph--last">
            A straightforward way to implement this idea is to combine sampling from the
            reference policy with optimization of actions against the learned critic.
            Previous successful attempts to use test-time compute in this way have mostly
            relied on a best-of-N sampling strategy …
          </p>
        </article>
      </div>
    </section>
  );
}
