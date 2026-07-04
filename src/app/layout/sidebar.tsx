import { Link } from '@tanstack/react-router';
import {
  ChevronDown,
  ChevronRight,
  FileText,
  Folder,
  Globe,
  MessageSquare,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  Sun,
} from 'lucide-react';
import { useState } from 'react';

import { useI18n } from '../../shared/i18n/i18n-context';
import { LogoMark } from '../../shared/ui/logo-mark';
import type { ThemeMode } from './app-layout';

interface SidebarProps {
  theme: ThemeMode;
  onThemeChange: (theme: ThemeMode) => void;
  onOpenSettings: () => void;
}

const projects = [
  {
    id: 'multimodal-papers',
    name: 'Multimodal Papers',
    items: [
      'Visual Grounding in Multimodal Foundation Models',
      'CLIP-Beyond: Unified Embeddings',
      'Audio-Visual Pretraining Survey',
    ],
  },
  {
    id: 'reasoning-benchmarks',
    name: 'Reasoning Benchmarks',
    items: ['RAG-Bench v2', 'Chain-of-Agents Eval Harness'],
  },
] as const;

export function Sidebar({ theme, onThemeChange, onOpenSettings }: SidebarProps) {
  const { t } = useI18n();
  const [isCollapsed, setIsCollapsed] = useState(false);
  const [areProjectsOpen, setAreProjectsOpen] = useState(true);
  const [openProjects, setOpenProjects] = useState<Record<string, boolean>>({});

  const nextTheme = theme === 'light' ? 'dark' : 'light';
  const ThemeIcon = theme === 'light' ? Moon : Sun;
  const CollapseIcon = isCollapsed ? PanelLeftOpen : PanelLeftClose;
  const themeLabel = theme === 'light' ? t('nav.darkMode') : t('nav.lightMode');

  const toggleProject = (id: string) => {
    setOpenProjects((current) => ({
      ...current,
      [id]: !current[id],
    }));
  };

  return (
    <aside
      className={isCollapsed ? 'sidebar sidebar--collapsed' : 'sidebar'}
      aria-label="Application navigation"
    >
      <div className="sidebar__header">
        <Link to="/" className="sidebar__logo-link" aria-label="r-a home">
          <LogoMark />
        </Link>
        {!isCollapsed ? <div className="sidebar__header-spacer" /> : null}
        <button
          className="sidebar__icon-button"
          type="button"
          title={isCollapsed ? t('nav.expand') : t('nav.collapse')}
          aria-label={isCollapsed ? t('nav.expand') : t('nav.collapse')}
          onClick={() => setIsCollapsed((value) => !value)}
        >
          <CollapseIcon aria-hidden="true" size={18} strokeWidth={2} />
        </button>
      </div>

      <nav className="sidebar__nav" aria-label="Primary navigation">
        <Link
          to="/"
          className="sidebar__nav-button sidebar__nav-button--active"
          title={t('nav.search')}
          aria-label={t('nav.search')}
          activeProps={{ className: 'sidebar__nav-button sidebar__nav-button--active' }}
        >
          <Globe aria-hidden="true" size={18} strokeWidth={2} />
          {!isCollapsed ? <span>{t('nav.search')}</span> : null}
        </Link>

        <div className="sidebar__group">
          <button
            className="sidebar__nav-button"
            type="button"
            title={t('nav.projects')}
            aria-expanded={!isCollapsed && areProjectsOpen}
            onClick={() => {
              if (isCollapsed) {
                setIsCollapsed(false);
                return;
              }

              setAreProjectsOpen((value) => !value);
            }}
          >
            <Folder aria-hidden="true" size={18} strokeWidth={2} />
            {!isCollapsed ? (
              <>
                <span>{t('nav.projects')}</span>
                {areProjectsOpen ? (
                  <ChevronDown className="sidebar__chevron" aria-hidden="true" size={16} />
                ) : (
                  <ChevronRight className="sidebar__chevron" aria-hidden="true" size={16} />
                )}
              </>
            ) : null}
          </button>

          {!isCollapsed && areProjectsOpen ? (
            <div className="sidebar__project-tree">
              {projects.map((project) => {
                const isOpen = !!openProjects[project.id];
                const ProjectChevron = isOpen ? ChevronDown : ChevronRight;

                return (
                  <div className="sidebar__project-group" key={project.id}>
                    <button
                      className="sidebar__project-button"
                      type="button"
                      onClick={() => toggleProject(project.id)}
                      aria-expanded={isOpen}
                    >
                      <ProjectChevron aria-hidden="true" size={14} strokeWidth={2} />
                      <span>{project.name}</span>
                      <span className="sidebar__project-count">{project.items.length}</span>
                    </button>
                    {isOpen ? (
                      <div className="sidebar__project-items">
                        {project.items.map((item) => (
                          <a className="sidebar__project-item" href="#" key={item}>
                            <FileText aria-hidden="true" size={13} strokeWidth={2} />
                            <span>{item}</span>
                          </a>
                        ))}
                      </div>
                    ) : null}
                  </div>
                );
              })}
            </div>
          ) : null}
        </div>
      </nav>

      <div className="sidebar__footer">
        <button
          className="sidebar__nav-button"
          type="button"
          title={t('nav.settings')}
          onClick={onOpenSettings}
        >
          <Settings aria-hidden="true" size={18} strokeWidth={2} />
          {!isCollapsed ? <span>{t('nav.settings')}</span> : null}
        </button>

        <button className="sidebar__nav-button" type="button" title={t('nav.feedback')}>
          <MessageSquare aria-hidden="true" size={18} strokeWidth={2} />
          {!isCollapsed ? <span>{t('nav.feedback')}</span> : null}
        </button>

        <button
          className="sidebar__nav-button"
          type="button"
          title={themeLabel}
          onClick={() => onThemeChange(nextTheme)}
        >
          <ThemeIcon aria-hidden="true" size={18} strokeWidth={2} />
          {!isCollapsed ? <span>{themeLabel}</span> : null}
        </button>
      </div>
    </aside>
  );
}
