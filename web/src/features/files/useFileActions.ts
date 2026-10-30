import { useState } from "react";
import {
  createFolder,
  deleteFile,
  renameFile,
  type BrowseEntry,
} from "../../lib/api";
import { useToast } from "../../components/ToastProvider";
import { joinPath } from "./paths";

type Reload = (
  target: string,
  until?: (list: BrowseEntry[]) => boolean,
) => Promise<void>;

export function useFileActions(path: string, load: Reload) {
  const notify = useToast();
  const [renameTarget, setRenameTarget] = useState<BrowseEntry | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<BrowseEntry | null>(null);
  const [mkdirOpen, setMkdirOpen] = useState(false);
  const [nameInput, setNameInput] = useState("");
  const [busy, setBusy] = useState(false);

  const openRename = (entry: BrowseEntry) => {
    setNameInput(entry.name);
    setRenameTarget(entry);
  };

  const openMkdir = () => {
    setNameInput("");
    setMkdirOpen(true);
  };

  const submitRename = async () => {
    if (!renameTarget) return;
    const name = nameInput.trim();
    if (!name) {
      notify("名称不能为空", "warn");
      return;
    }
    if (name === renameTarget.name) {
      setRenameTarget(null);
      return;
    }
    setBusy(true);
    try {
      await renameFile(joinPath(path, renameTarget.name), name);
      notify(`已重命名为 ${name}`, "ok");
      setRenameTarget(null);
      await load(path, (list) => list.some((entry) => entry.name === name));
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "重命名失败", "error");
    } finally {
      setBusy(false);
    }
  };

  const submitDelete = async () => {
    if (!deleteTarget) return;
    setBusy(true);
    try {
      await deleteFile(joinPath(path, deleteTarget.name));
      notify(`已移入回收站：${deleteTarget.name}`, "ok");
      const removedFID = deleteTarget.fid;
      setDeleteTarget(null);
      await load(path, (list) => !list.some((entry) => entry.fid === removedFID));
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "删除失败", "error");
    } finally {
      setBusy(false);
    }
  };

  const submitMkdir = async () => {
    const name = nameInput.trim();
    if (!name) {
      notify("文件夹名称不能为空", "warn");
      return;
    }
    if (name.includes("/")) {
      notify("文件夹名称不能包含 /", "warn");
      return;
    }
    setBusy(true);
    try {
      await createFolder(joinPath(path, name));
      notify(`已新建文件夹 ${name}`, "ok");
      setMkdirOpen(false);
      await load(path, (list) => list.some((entry) => entry.name === name));
    } catch (cause) {
      notify(cause instanceof Error ? cause.message : "新建文件夹失败", "error");
    } finally {
      setBusy(false);
    }
  };

  return {
    renameTarget,
    openRename,
    closeRename: () => setRenameTarget(null),
    deleteTarget,
    setDeleteTarget,
    closeDelete: () => setDeleteTarget(null),
    mkdirOpen,
    setMkdirOpen,
    openMkdir,
    nameInput,
    setNameInput,
    busy,
    submitRename,
    submitDelete,
    submitMkdir,
  };
}

export type FileActions = ReturnType<typeof useFileActions>;
