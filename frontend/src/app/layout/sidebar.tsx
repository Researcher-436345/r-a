import { Link, useNavigate } from '@tanstack/react-router';
import {
  BookMarked,
  ChevronDown,
  ChevronRight,
  FileText,
  Folder,
  Globe,
  LogOut,
  MessageSquare,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  Settings,
  Sun,
  type LucideIcon,
} from 'lucide-react';
import { useEffect, useState } from 'react';

import { clearTokens } from '../../features/auth/token-storage';
import { useI18n } from '../../shared/i18n/i18n-context';
import { LogoMark } from '../../shared/ui/logo-mark';
import type { ThemeMode } from '../../shared/theme/theme-context';

interface SidebarProps {
  theme: ThemeMode;
  onThemeChange: (theme: ThemeMode) => void;
  onOpenSettings: () => void;
  defaultCollapsed?: boolean;
  defaultProjectsOpen?: boolean;
  projects?: readonly SidebarProject[];
}

export interface SidebarProject {
  id: string;
  name: string;
  items: readonly string[];
}

interface SidebarPrimaryNavProps {
  isCollapsed: boolean;
  areProjectsOpen: boolean;
  projects: readonly SidebarProject[];
  openProjects: Record<string, boolean>;
  searchLabel: string;
  projectsLabel: string;
  onExpandSidebar: () => void;
  onToggleProjects: () => void;
  onToggleProject: (id: string) => void;
}

interface SidebarFooterProps {
  isCollapsed: boolean;
  settingsLabel: string;
  feedbackLabel: string;
  themeLabel: string;
  logoutLabel: string;
  ThemeIcon: LucideIcon;
  onOpenSettings: () => void;
  onToggleTheme: () => void;
  onLogout: () => void;
}

const homeSidebarProjects = [
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

export const readerSidebarProjects = [
  {
    id: 'rl',
    name: 'Reinforcement Learning',
    items: ['Q-Guided Flow Policies', 'IDQL as Actor-Critic', 'Diffusion Policy'],
  },
  {
    id: 'mm',
    name: 'Multimodal',
    items: ['Visual Grounding at Scale', 'CLIP-Beyond'],
  },
] as const;

export function Sidebar({
  theme,
  onThemeChange,
  onOpenSettings,
  defaultCollapsed = false,
  defaultProjectsOpen = true,
  projects = homeSidebarProjects,
}: SidebarProps) {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [isCollapsed, setIsCollapsed] = useState(defaultCollapsed);
  const [areProjectsOpen, setAreProjectsOpen] = useState(defaultProjectsOpen);
  const [openProjects, setOpenProjects] = useState<Record<string, boolean>>({});

  const nextTheme = theme === 'light' ? 'dark' : 'light';
  const ThemeIcon = theme === 'light' ? Moon : Sun;
  const CollapseIcon = isCollapsed ? PanelLeftOpen : PanelLeftClose;
  const themeLabel = theme === 'light' ? t('nav.darkMode') : t('nav.lightMode');

  const handleLogout = () => {
    clearTokens();
    void navigate({ to: '/login' });
  };

  useEffect(() => {
    setIsCollapsed(defaultCollapsed);
  }, [defaultCollapsed]);

  useEffect(() => {
    setAreProjectsOpen(defaultProjectsOpen);
  }, [defaultProjectsOpen]);

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

      <SidebarPrimaryNav
        isCollapsed={isCollapsed}
        areProjectsOpen={areProjectsOpen}
        projects={projects}
        openProjects={openProjects}
        searchLabel={t('nav.search')}
        projectsLabel={t('nav.projects')}
        onExpandSidebar={() => setIsCollapsed(false)}
        onToggleProjects={() => setAreProjectsOpen((value) => !value)}
        onToggleProject={toggleProject}
      />

      <SidebarFooter
        isCollapsed={isCollapsed}
        settingsLabel={t('nav.settings')}
        feedbackLabel={t('nav.feedback')}
        themeLabel={themeLabel}
        logoutLabel="Выйти"
        ThemeIcon={ThemeIcon}
        onOpenSettings={onOpenSettings}
        onToggleTheme={() => onThemeChange(nextTheme)}
        onLogout={handleLogout}
      />
    </aside>
  );
}

function SidebarPrimaryNav({
  isCollapsed,
  areProjectsOpen,
  projects,
  openProjects,
  searchLabel,
  projectsLabel,
  onExpandSidebar,
  onToggleProjects,
  onToggleProject,
}: SidebarPrimaryNavProps) {
  return (
    <nav className="sidebar__nav" aria-label="Primary navigation">
      <Link
        to="/"
        className="sidebar__nav-button"
        title={searchLabel}
        aria-label={searchLabel}
        activeProps={{ className: 'sidebar__nav-button sidebar__nav-button--active' }}
      >
        <Globe aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{searchLabel}</span> : null}
      </Link>

      <Link
        to="/library"
        className="sidebar__nav-button"
        title="Библиотека"
        aria-label="Библиотека"
        activeProps={{ className: 'sidebar__nav-button sidebar__nav-button--active' }}
      >
        <BookMarked aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>Библиотека</span> : null}
      </Link>

      <div className="sidebar__group">
        <button
          className="sidebar__nav-button"
          type="button"
          title={projectsLabel}
          aria-expanded={!isCollapsed && areProjectsOpen}
          onClick={() => {
            if (isCollapsed) {
              onExpandSidebar();
              return;
            }

            onToggleProjects();
          }}
        >
          <Folder aria-hidden="true" size={18} strokeWidth={2} />
          {!isCollapsed ? (
            <>
              <span>{projectsLabel}</span>
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
            {projects.map((project) => (
              <SidebarProjectGroup
                key={project.id}
                project={project}
                isOpen={!!openProjects[project.id]}
                onToggle={() => onToggleProject(project.id)}
              />
            ))}
          </div>
        ) : null}
      </div>
    </nav>
  );
}

function SidebarProjectGroup({
  project,
  isOpen,
  onToggle,
}: {
  project: SidebarProject;
  isOpen: boolean;
  onToggle: () => void;
}) {
  const ProjectChevron = isOpen ? ChevronDown : ChevronRight;

  return (
    <div className="sidebar__project-group">
      <button
        className="sidebar__project-button"
        type="button"
        onClick={onToggle}
        aria-expanded={isOpen}
      >
        <ProjectChevron aria-hidden="true" size={14} strokeWidth={2} />
        <span>{project.name}</span>
        <span className="sidebar__project-count">{project.items.length}</span>
      </button>
      {isOpen ? (
        <div className="sidebar__project-items">
          {project.items.map((item) => (
            <Link className="sidebar__project-item" to="/reader" key={item}>
              <FileText aria-hidden="true" size={13} strokeWidth={2} />
              <span>{item}</span>
            </Link>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function SidebarFooter({
  isCollapsed,
  settingsLabel,
  feedbackLabel,
  themeLabel,
  logoutLabel,
  ThemeIcon,
  onOpenSettings,
  onToggleTheme,
  onLogout,
}: SidebarFooterProps) {
  return (
    <div className="sidebar__footer">
      <button
        className="sidebar__nav-button"
        type="button"
        title={settingsLabel}
        onClick={onOpenSettings}
      >
        <Settings aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{settingsLabel}</span> : null}
      </button>

      <button className="sidebar__nav-button" type="button" title={feedbackLabel}>
        <MessageSquare aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{feedbackLabel}</span> : null}
      </button>

      <button
        className="sidebar__nav-button"
        type="button"
        title={themeLabel}
        onClick={onToggleTheme}
      >
        <ThemeIcon aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{themeLabel}</span> : null}
      </button>

      <button
        className="sidebar__nav-button"
        type="button"
        title={logoutLabel}
        onClick={onLogout}
      >
        <LogOut aria-hidden="true" size={18} strokeWidth={2} />
        {!isCollapsed ? <span>{logoutLabel}</span> : null}
      </button>
    </div>
  );
}
