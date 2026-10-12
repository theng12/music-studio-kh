const audio = /\.(mp3|m4a|aac|flac|ogg|opus|wav)$/i;
export async function droppedFiles(data: DataTransfer): Promise<File[]> {
  const files: File[] = [];
  const walk = async (entry: any): Promise<void> => {
    if (!entry || entry.name.startsWith(".")) return;
    if (entry.isFile) {
      if (!audio.test(entry.name)) return;
      const file = await new Promise<File>((resolve, reject) =>
        entry.file(resolve, reject),
      );
      files.push(file);
      if (files.length > 10000)
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
        for (const child of children) await walk(child);
      }
    }
  };
  const entries = [...data.items]
    .map((item) => (item as any).webkitGetAsEntry?.())
    .filter(Boolean);
  if (entries.length) {
    for (const entry of entries) await walk(entry);
    return files;
  }
  return [...data.files].filter((file) => audio.test(file.name));
}
