import { Link, useNavigate, useSearch } from '@tanstack/react-router';
import {
  Check,
  ChevronDown,
  ChevronRight,
  FilePlus2,
  Folder,
  FolderOpen,
  FolderPlus,
  LoaderCircle,
  Plus,
  RotateCcw,
  Trash2,
  X,
} from 'lucide-react';
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';

import {
  createLibraryFolder,
  deleteLibraryFolder,
  fetchLibrary,
  fetchLibraryFolders,
  removeFromLibrary,
  retryPdf,
  type LibraryFolder,
  type LibraryItem,
} from '../../features/library/api';
import { LibraryFolderIcon } from '../../features/library/folder-icons';
import { ApiError } from '../../shared/api/client';
import { RichText } from '../../shared/ui/rich-text';

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

interface FolderRowProps {
  folder: FolderNode;
  depth: number;
  selectedId: string;
  expanded: Record<string, boolean>;
  onSelect: (id: string) => void;
  onToggle: (id: string) => void;
  onAddChild: (id: string) => void;
  onDelete: (folder: LibraryFolder) => void;
  deletingId: string | null;
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
  onDelete,
  deletingId,
  createParentId,
  createForm,
}: FolderRowProps) {
  const isOpen = expanded[folder.id] ?? true;
  const hasChildren = folder.children.length > 0;
  return (
    <div className="library-folders__branch">
      <div
        className={
          `library-folders__row${selectedId === folder.id ? ' library-folders__row--active' : ''}${
            hasChildren ? ' library-folders__row--has-children' : ''
          }`
        }
        style={{ paddingLeft: `${10 + depth * 16}px` }}
      >
        <button
          className="library-folders__select"
          type="button"
          onClick={() => {
            onSelect(folder.id);
            if (hasChildren && (selectedId === folder.id || !isOpen)) {
              onToggle(folder.id);
            }
          }}
          title={folder.name}
        >
          {folder.system_key ? (
            <LibraryFolderIcon folder={folder} size={16} />
          ) : selectedId === folder.id ? (
            <FolderOpen aria-hidden="true" size={16} strokeWidth={2} />
          ) : (
            <Folder aria-hidden="true" size={16} strokeWidth={2} />
          )}
          <span>{folder.name}</span>
        </button>
        <div className="library-folders__actions">
          <button
            className="library-folders__add-child"
            type="button"
            onClick={() => onAddChild(folder.id)}
            title={`Новая папка внутри «${folder.name}»`}
            aria-label={`Новая папка внутри «${folder.name}»`}
          >
            <FolderPlus aria-hidden="true" size={14} strokeWidth={2} />
          </button>
          {!folder.system_key ? (
            <button
              className="library-folders__delete"
              type="button"
              onClick={() => onDelete(folder)}
              disabled={deletingId === folder.id}
              title={`Удалить папку «${folder.name}»`}
              aria-label={`Удалить папку «${folder.name}»`}
            >
              {deletingId === folder.id ? (
                <LoaderCircle className="spin" aria-hidden="true" size={14} />
              ) : (
                <Trash2 aria-hidden="true" size={14} strokeWidth={2} />
              )}
            </button>
          ) : null}
        </div>
        {hasChildren ? (
          <button
            className="library-folders__toggle"
            type="button"
            onClick={() => onToggle(folder.id)}
            aria-label={isOpen ? 'Свернуть папку' : 'Развернуть папку'}
            aria-expanded={isOpen}
          >
            {isOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
          </button>
        ) : null}
        <span className="library-folders__count">{folder.article_count}</span>
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
              onDelete={onDelete}
              deletingId={deletingId}
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
  const [deletingPaperId, setDeletingPaperId] = useState<string | null>(null);
  const [deletingFolderId, setDeletingFolderId] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [createParentId, setCreateParentId] = useState<string | null | undefined>(undefined);
  const [newFolderName, setNewFolderName] = useState('');
  const [isCreatingFolder, setIsCreatingFolder] = useState(false);
  const [folderError, setFolderError] = useState<string | null>(null);
  const folderInputRef = useRef<HTMLInputElement>(null);

  const tree = useMemo(() => folderTree(folders), [folders]);
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

  const handleDeletePaper = async (item: LibraryItem) => {
    if (!window.confirm(`Удалить «${item.paper.title}» из библиотеки?`)) {
      return;
    }
    setDeletingPaperId(item.paper.id);
    setError(null);
    try {
      await removeFromLibrary(item.paper.id);
      setItems((current) => current.filter((candidate) => candidate.id !== item.id));
      setTotal((current) => Math.max(0, current - 1));
      await refreshFolders(selectedFolderId);
    } catch (err) {
      setError(err instanceof ApiError ? err.detail : 'Не удалось удалить статью');
    } finally {
      setDeletingPaperId(null);
    }
  };

  const handleDeleteFolder = async (folder: LibraryFolder) => {
    const confirmed = window.confirm(
      `Удалить папку «${folder.name}» вместе со всеми вложенными папками? Статьи будут перемещены в «Другое».`,
    );
    if (!confirmed) {
      return;
    }

    const byId = new Map(folders.map((item) => [item.id, item]));
    let selectedInsideDeletedFolder = false;
    let cursor = byId.get(selectedFolderId);
    while (cursor) {
      if (cursor.id === folder.id) {
        selectedInsideDeletedFolder = true;
        break;
      }
      cursor = cursor.parent_id ? byId.get(cursor.parent_id) : undefined;
    }

    setDeletingFolderId(folder.id);
    setFolderError(null);
    try {
      await deleteLibraryFolder(folder.id);
      const fallbackId = folders.find((item) => item.system_key === 'other')?.id;
      if (selectedInsideDeletedFolder && fallbackId) {
        selectFolder(fallbackId);
        await refreshFolders(fallbackId);
      } else {
        await refreshFolders(selectedFolderId);
      }
    } catch (err) {
      setFolderError(err instanceof ApiError ? err.detail : 'Не удалось удалить папку');
    } finally {
      setDeletingFolderId(null);
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
              onDelete={(folder) => void handleDeleteFolder(folder)}
              deletingId={deletingFolderId}
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
            const identifier = item.paper.arxiv_id
              ? `arXiv:${item.paper.arxiv_id}`
              : item.paper.doi
                ? `DOI:${item.paper.doi}`
                : null;

            return (
              <article className="library-card" key={item.id}>
                <div className="library-card__body">
                  <div className="library-card__top">
                    <Link
                      className="library-card__title"
                      to="/reader/$paperId"
                      params={{ paperId: item.paper.id }}
                    >
                      {item.paper.title}
                    </Link>
                    <span className="library-card__actions">
                      {identifier ? (
                        <span className="library-card__identifier">{identifier}</span>
                      ) : null}
                      <button
                        className="library-card__delete"
                        type="button"
                        disabled={deletingPaperId === item.paper.id}
                        onClick={() => void handleDeletePaper(item)}
                        title="Удалить из библиотеки"
                        aria-label={`Удалить «${item.paper.title}» из библиотеки`}
                      >
                        {deletingPaperId === item.paper.id ? (
                          <LoaderCircle className="spin" aria-hidden="true" size={15} />
                        ) : (
                          <Trash2 aria-hidden="true" size={15} strokeWidth={2} />
                        )}
                      </button>
                    </span>
                  </div>
                  {authors ? <div className="library-card__authors">{authors}</div> : null}
                  {status === 'failed' ? (
                    <button
                      type="button"
                      className="library-card__retry"
                      disabled={retryingId === item.paper.id}
                      onClick={() => void handleRetry(item.paper.id)}
                    >
                      <RotateCcw size={14} strokeWidth={2} />
                      {retryingId === item.paper.id ? 'Повтор…' : 'Повторить обработку PDF'}
                    </button>
                  ) : null}
                  {status === 'failed' && version?.error_message ? (
                    <p className="library-card__error">{version.error_message}</p>
                  ) : null}
                  {item.paper.abstract ? (
                    <RichText className="library-card__abstract" compact>
                      {item.paper.abstract}
                    </RichText>
                  ) : null}
                </div>
              </article>
            );
          })}
        </div>
      </section>
    </div>
  );
}
