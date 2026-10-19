import { ImportItem } from "./api";

const audio = /\.(mp3|m4a|aac|flac|ogg|opus|wav)$/i;
const coverArt = /^(cover|folder|artwork|front)\.(jpg|jpeg|png|webp)$/i;

export function isImportableFile(name: string, inFolder = false): boolean {
  return audio.test(name) || (inFolder && coverArt.test(name));
}

export async function droppedFiles(data: DataTransfer): Promise<ImportItem[]> {
  const items: ImportItem[] = [];
  const walk = async (entry: any, parentPath = ""): Promise<void> => {
    if (!entry || entry.name.startsWith(".")) return;
    const relPath = parentPath ? `${parentPath}/${entry.name}` : entry.name;
    if (entry.isFile) {
      if (!isImportableFile(entry.name, Boolean(parentPath))) return;
      const file = await new Promise<File>((resolve, reject) =>
        entry.file(resolve, reject),
      );
      items.push({ file, relPath });
      if (items.length > 10000)
        throw new Error("Import at most 10,000 songs at a time.");
      return;
    }
    if (entry.isDirectory) {
      const reader = entry.createReader();
      for (;;) {
        const children: any[] = await new Promise((resolve, reject) =>
          reader.readEntries(resolve, reject),
        );
        if (!children.length) break;
        for (const child of children) await walk(child, relPath);
      }
    }
  };
  const entries = [...data.items]
    .map((item) => (item as any).webkitGetAsEntry?.())
    .filter(Boolean);
  if (entries.length) {
    for (const entry of entries) await walk(entry);
    return items;
  }
  return [...data.files]
    .filter((file) =>
      isImportableFile(file.name, Boolean((file as any).webkitRelativePath)),
    )
    .map((file) => ({
      file,
      relPath: (file as any).webkitRelativePath || file.name,
    }));
}

