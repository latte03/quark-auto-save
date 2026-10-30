import { useCallback, useRef, useState } from "react";
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
  // busy 状态要等渲染后才生效，回车连按两下会重复提交，用 ref 做同步锁。
  const busyRef = useRef(false);

  const run = async (task: () => Promise<void>) => {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    try {
      await task();
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  };

  const openRename = (entry: BrowseEntry) => {
    setNameInput(entry.name);
    setRenameTarget(entry);
  };

  const openMkdir = () => {
    setNameInput("");
    setMkdirOpen(true);
  };

  const submitRename = useCallback(async () => {
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
    await run(async () => {
      try {
        await renameFile(joinPath(path, renameTarget.name), name);
        notify(`已重命名为 ${name}`, "ok");
        setRenameTarget(null);
        await load(path, (list) => list.some((entry) => entry.name === name));
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "重命名失败", "error");
      }
    });
  }, [load, nameInput, notify, path, renameTarget]);

  const submitDelete = useCallback(async () => {
    if (!deleteTarget) return;
    await run(async () => {
      try {
        await deleteFile(joinPath(path, deleteTarget.name));
        notify(`已移入回收站：${deleteTarget.name}`, "ok");
        const removedFID = deleteTarget.fid;
        setDeleteTarget(null);
        await load(path, (list) => !list.some((entry) => entry.fid === removedFID));
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "删除失败", "error");
      }
    });
  }, [deleteTarget, load, notify, path]);

  const submitMkdir = useCallback(async () => {
    const name = nameInput.trim();
    if (!name) {
      notify("文件夹名称不能为空", "warn");
      return;
    }
    if (name.includes("/")) {
      notify("文件夹名称不能包含 /", "warn");
      return;
    }
    await run(async () => {
      try {
        await createFolder(joinPath(path, name));
        notify(`已新建文件夹 ${name}`, "ok");
        setMkdirOpen(false);
        await load(path, (list) => list.some((entry) => entry.name === name));
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "新建文件夹失败", "error");
      }
    });
  }, [load, nameInput, notify, path]);

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
