import { Archive, BookOpen, Clock3, Folder, type LucideIcon } from 'lucide-react';

import type { LibraryFolder } from './api';

const SYSTEM_FOLDER_ICONS: Record<string, LucideIcon> = {
  want_to_read: BookOpen,
  reading: Clock3,
  other: Archive,
};

export function LibraryFolderIcon({
  folder,
  size = 15,
}: {
  folder: LibraryFolder;
  size?: number;
}) {
  const Icon = folder.system_key ? SYSTEM_FOLDER_ICONS[folder.system_key] ?? Folder : Folder;
  return <Icon aria-hidden="true" size={size} strokeWidth={2} />;
}

export function flattenLibraryFolders(folders: LibraryFolder[]) {
  const children = new Map<string | null, LibraryFolder[]>();
  for (const folder of folders) {
    const siblings = children.get(folder.parent_id) ?? [];
    siblings.push(folder);
    children.set(folder.parent_id, siblings);
  }

  const result: Array<{ folder: LibraryFolder; depth: number }> = [];
  const visit = (parentId: string | null, depth: number) => {
    for (const folder of children.get(parentId) ?? []) {
      result.push({ folder, depth });
      visit(folder.id, depth + 1);
    }
  };
  visit(null, 0);
  return result;
}
