import { Link, useNavigate, useSearch } from '@tanstack/react-router';
import {
  Archive,
  BookOpen,
  Check,
  ChevronDown,
  ChevronRight,
  Clock3,
  FilePlus2,
  Folder,
  FolderOpen,
  FolderPlus,
  LoaderCircle,
  Plus,
  RotateCcw,
  X,
  type LucideIcon,
} from 'lucide-react';
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';

import {
  createLibraryFolder,
  fetchLibrary,
  fetchLibraryFolders,
  patchLibraryItem,
  retryPdf,
  type LibraryFolder,
  type LibraryItem,
} from '../../features/library/api';
import { ApiError } from '../../shared/api/client';
import { RichText } from '../../shared/ui/rich-text';

const STATUS_LABELS: Record<string, string> = {
  ready: 'PDF готов',
  processing: 'Обрабатывается',
  uploading: 'Загрузка',
  failed: 'Ошибка PDF',
};

const SYSTEM_FOLDER_ICONS: Record<string, LucideIcon> = {
  want_to_read: BookOpen,
  reading: Clock3,
  other: Archive,
};

interface FolderNode extends LibraryFolder {
  children: FolderNode[];
}

function folderTree(folders: LibraryFolder[]) {
  const nodes = new Map<string, FolderNode>();
  const roots: FolderNode[] = [];
  folders.forEach((folder) => nodes.set(folder.id, { ...folder, children: [] }));
  folders.forEach((folder) => {
    const node = nodes.get(folder.id);
    if (!node) {
      return;
    }
    const parent = folder.parent_id ? nodes.get(folder.parent_id) : undefined;
    if (parent) {
      parent.children.push(node);
    } else {
      roots.push(node);
    }
  });
  return roots;
}

function flatFolders(nodes: FolderNode[], depth = 0): Array<{ folder: FolderNode; depth: number }> {
  return nodes.flatMap((folder) => [
    { folder, depth },
    ...flatFolders(folder.children, depth + 1),
  ]);
}

interface FolderRowProps {
  folder: FolderNode;
  depth: number;
  selectedId: string;
  expanded: Record<string, boolean>;
  onSelect: (id: string) => void;
  onToggle: (id: string) => void;
  onAddChild: (id: string) => void;
  createParentId: string | null | undefined;
  createForm: React.ReactNode;
}

function FolderRow({
  folder,
  depth,
  selectedId,
  expanded,
  onSelect,
  onToggle,
  onAddChild,
  createParentId,
  createForm,
}: FolderRowProps) {
  const isOpen = expanded[folder.id] ?? true;
  const hasChildren = folder.children.length > 0;
  const Icon = folder.system_key
    ? SYSTEM_FOLDER_ICONS[folder.system_key] ?? Folder
    : selectedId === folder.id
      ? FolderOpen
      : Folder;

  return (
    <div className="library-folders__branch">
      <div
        className={
          selectedId === folder.id
            ? 'library-folders__row library-folders__row--active'
            : 'library-folders__row'
        }
        style={{ paddingLeft: `${10 + depth * 16}px` }}
      >
        <button
          className="library-folders__toggle"
          type="button"
          onClick={() => onToggle(folder.id)}
          aria-label={isOpen ? 'Свернуть папку' : 'Развернуть папку'}
          aria-expanded={hasChildren ? isOpen : undefined}
          disabled={!hasChildren}
        >
          {hasChildren ? (
            isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />
          ) : (
            <span aria-hidden="true" />
          )}
        </button>
        <button
          className="library-folders__select"
          type="button"
          onClick={() => onSelect(folder.id)}
          title={folder.name}
        >
          <Icon aria-hidden="true" size={16} strokeWidth={2} />
          <span>{folder.name}</span>
          <span className="library-folders__count">{folder.article_count}</span>
        </button>
        <button
          className="library-folders__add-child"
          type="button"
          onClick={() => onAddChild(folder.id)}
          title={`Новая папка внутри «${folder.name}»`}
          aria-label={`Новая папка внутри «${folder.name}»`}
        >
          <FolderPlus aria-hidden="true" size={14} strokeWidth={2} />
        </button>
      </div>
      {createParentId === folder.id ? createForm : null}
      {hasChildren && isOpen ? (
        <div className="library-folders__children">
          {folder.children.map((child) => (
            <FolderRow
              key={child.id}
              folder={child}
              depth={depth + 1}
              selectedId={selectedId}
              expanded={expanded}
              onSelect={onSelect}
              onToggle={onToggle}
              onAddChild={onAddChild}
              createParentId={createParentId}
              createForm={createForm}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

export function LibraryPage() {
  const navigate = useNavigate();
  const routeSearch = useSearch({ strict: false }) as { folder?: string };
  const [folders, setFolders] = useState<LibraryFolder[]>([]);
  const [selectedFolderId, setSelectedFolderId] = useState(routeSearch.folder ?? '');
  const [items, setItems] = useState<LibraryItem[]>([]);
  const [total, setTotal] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [retryingId, setRetryingId] = useState<string | null>(null);
  const [movingId, setMovingId] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [createParentId, setCreateParentId] = useState<string | null | undefined>(undefined);
  const [newFolderName, setNewFolderName] = useState('');
  const [isCreatingFolder, setIsCreatingFolder] = useState(false);
  const [folderError, setFolderError] = useState<string | null>(null);
  const folderInputRef = useRef<HTMLInputElement>(null);

  const tree = useMemo(() => folderTree(folders), [folders]);
  const folderOptions = useMemo(() => flatFolders(tree), [tree]);
  const selectedFolder = folders.find((folder) => folder.id === selectedFolderId) ?? null;

  const selectFolder = (folderId: string) => {
    setSelectedFolderId(folderId);
    void navigate({
      to: '/library',
      search: { folder: folderId },
      replace: true,
    });
  };

  const refreshFolders = async (preferredId?: string) => {
    const response = await fetchLibraryFolders();
    const nextFolders = response.items ?? [];
    setFolders(nextFolders);
    setSelectedFolderId((current) => {
      const candidate = preferredId ?? current;
      if (candidate && nextFolders.some((folder) => folder.id === candidate)) {
        return candidate;
      }
      return (
        nextFolders.find((folder) => folder.system_key === 'want_to_read')?.id ??
        nextFolders[0]?.id ??
        ''
      );
    });
  };

  const loadLibrary = async (folderId: string) => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await fetchLibrary(1, 100, folderId);
      setItems(data.items ?? []);
      setTotal(data.total ?? 0);
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : 'Не удалось загрузить библиотеку');
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    void refreshFolders().catch((err: unknown) => {
      setError(err instanceof ApiError ? err.detail : 'Не удалось загрузить папки');
      setIsLoading(false);
    });
  }, []);

  useEffect(() => {
    if (selectedFolderId) {
      void loadLibrary(selectedFolderId);
    }
  }, [selectedFolderId]);

  useEffect(() => {
    if (createParentId !== undefined) {
      window.requestAnimationFrame(() => folderInputRef.current?.focus());
    }
  }, [createParentId]);

  const beginCreateFolder = (parentId: string | null) => {
    setFolderError(null);
    setNewFolderName('');
    setCreateParentId(parentId);
    if (parentId) {
      setExpanded((current) => ({ ...current, [parentId]: true }));
    }
  };

  const cancelCreateFolder = () => {
    setCreateParentId(undefined);
    setNewFolderName('');
    setFolderError(null);
  };

  const submitFolder = async (event: FormEvent) => {
    event.preventDefault();
    const name = newFolderName.trim();
    if (!name || createParentId === undefined) {
      return;
    }
    setIsCreatingFolder(true);
    setFolderError(null);
    try {
      const created = await createLibraryFolder(name, createParentId);
      cancelCreateFolder();
      if (created.parent_id) {
        setExpanded((current) => ({ ...current, [created.parent_id as string]: true }));
      }
      selectFolder(created.id);
      await refreshFolders(created.id);
    } catch (err) {
      setFolderError(err instanceof ApiError ? err.detail : 'Не удалось создать папку');
    } finally {
      setIsCreatingFolder(false);
    }
  };

  const handleRetry = async (paperId: string) => {
    setRetryingId(paperId);
    setError(null);
    try {
      await retryPdf(paperId);
      await loadLibrary(selectedFolderId);
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : 'Не удалось перезапустить обработку PDF');
    } finally {
      setRetryingId(null);
    }
  };

  const movePaper = async (paperId: string, folderId: string) => {
    if (!folderId || folderId === selectedFolderId) {
      return;
    }
    setMovingId(paperId);
    setError(null);
    try {
      await patchLibraryItem(paperId, { folder_id: folderId });
      setItems((current) => current.filter((item) => item.paper.id !== paperId));
      setTotal((current) => Math.max(0, current - 1));
      await refreshFolders(selectedFolderId);
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : 'Не удалось переместить статью');
    } finally {
      setMovingId(null);
    }
  };

  const createForm = (
    <form className="library-folders__create-form" onSubmit={(event) => void submitFolder(event)}>
      <input
        ref={folderInputRef}
        value={newFolderName}
        onChange={(event) => setNewFolderName(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === 'Escape') {
            cancelCreateFolder();
          }
        }}
        maxLength={120}
        placeholder="Название папки"
        aria-label="Название новой папки"
        disabled={isCreatingFolder}
      />
      <button type="submit" disabled={!newFolderName.trim() || isCreatingFolder} title="Создать">
        {isCreatingFolder ? <LoaderCircle className="spin" size={14} /> : <Check size={14} />}
      </button>
      <button type="button" onClick={cancelCreateFolder} title="Отмена">
        <X size={14} />
      </button>
    </form>
  );

  return (
    <div className="library-workspace">
      <aside className="library-folders" aria-label="Папки библиотеки">
        <div className="library-folders__header">
          <button
            className="library-folders__new"
            type="button"
            onClick={() => beginCreateFolder(null)}
          >
            <Plus aria-hidden="true" size={15} strokeWidth={2} />
            <span>Новая папка</span>
          </button>
        </div>

        <nav className="library-folders__tree" aria-label="Дерево папок">
          {createParentId === null ? createForm : null}
          {tree.map((folder) => (
            <FolderRow
              key={folder.id}
              folder={folder}
              depth={0}
              selectedId={selectedFolderId}
              expanded={expanded}
              onSelect={selectFolder}
              onToggle={(id) =>
                setExpanded((current) => ({ ...current, [id]: !(current[id] ?? true) }))
              }
              onAddChild={(id) => beginCreateFolder(id)}
              createParentId={createParentId}
              createForm={createForm}
            />
          ))}
        </nav>
        {folderError ? <p className="library-folders__error">{folderError}</p> : null}
      </aside>

      <section className="library-page">
        <div className="library-page__header">
          <div>
            <h1>{selectedFolder?.name ?? 'Библиотека'}</h1>
            <p>{total > 0 ? `${total} статей` : 'В этой папке пока нет статей'}</p>
          </div>
          <Link
            className="library-page__add"
            to="/library/add"
            search={{ folder: selectedFolderId }}
            aria-label="Добавить статью"
            title="Добавить статью"
          >
            <FilePlus2 aria-hidden="true" size={20} strokeWidth={1.8} />
          </Link>
        </div>

        {isLoading ? (
          <div className="library-page__state">
            <LoaderCircle className="spin" size={18} />
            Загружаем…
          </div>
        ) : null}

        {error ? <div className="library-page__error">{error}</div> : null}

        {!isLoading && !error && items.length === 0 ? (
          <div className="library-page__empty">
            <FolderOpen aria-hidden="true" size={22} strokeWidth={1.7} />
            <p>Добавленные статьи появятся здесь.</p>
          </div>
        ) : null}

        <div className="library-list">
          {items.map((item) => {
            const authors = (item.paper.authors ?? []).map((author) => author.name).join(', ');
            const version = item.paper.latest_version;
            const status = version?.status ?? 'processing';
            const statusLabel = STATUS_LABELS[status] ?? status;

            return (
              <article className="library-card" key={item.id}>
                <div className="library-card__body">
                  <Link
                    className="library-card__title"
                    to="/reader/$paperId"
                    params={{ paperId: item.paper.id }}
                  >
                    {item.paper.title}
                  </Link>
                  {authors ? <div className="library-card__authors">{authors}</div> : null}
                  <div className="library-card__meta">
                    {item.paper.year ? <span>{item.paper.year}</span> : null}
                    {item.paper.arxiv_id ? <span>arXiv:{item.paper.arxiv_id}</span> : null}
                    {item.paper.doi ? <span>DOI:{item.paper.doi}</span> : null}
                    <span
                      className={`library-card__status library-card__status--${status}`}
                      title={version?.error_message ?? undefined}
                    >
                      {statusLabel}
                    </span>
                    {status === 'failed' ? (
                      <button
                        type="button"
                        className="library-card__retry"
                        disabled={retryingId === item.paper.id}
                        onClick={() => void handleRetry(item.paper.id)}
                      >
                        <RotateCcw size={14} strokeWidth={2} />
                        {retryingId === item.paper.id ? 'Повтор…' : 'Повторить'}
                      </button>
                    ) : null}
                  </div>
                  {status === 'failed' && version?.error_message ? (
                    <p className="library-card__error">{version.error_message}</p>
                  ) : null}
                  {item.paper.abstract ? (
                    <RichText className="library-card__abstract" compact>
                      {item.paper.abstract}
                    </RichText>
                  ) : null}
                </div>
                <label className="library-card__folder-select">
                  <span>Папка</span>
                  <select
                    value={item.folder_id ?? selectedFolderId}
                    disabled={movingId === item.paper.id}
                    onChange={(event) => void movePaper(item.paper.id, event.target.value)}
                  >
                    {folderOptions.map(({ folder, depth }) => (
                      <option value={folder.id} key={folder.id}>
                        {`${'— '.repeat(depth)}${folder.name}`}
                      </option>
                    ))}
                  </select>
                </label>
              </article>
            );
          })}
        </div>
      </section>
    </div>
  );
}
