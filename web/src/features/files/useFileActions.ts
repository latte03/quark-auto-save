import { useCallback, useRef, useState } from "react";
import {
  createFolder,
  deleteFile,
  renameFile,
  type BrowseEntry,
} from "../../lib/api";
import { useToast } from "../../components/ToastProvider";
import { joinPath } from "./paths";
import type { DirectoryCache } from "./useDirectoryCache";

type RenamedHandler = (from: string, to: string) => void;
type DeletedHandler = (path: string) => void;

export function useFileActions(path: string, cache: DirectoryCache) {
  const notify = useToast();
  const [renameTarget, setRenameTarget] = useState<BrowseEntry | null>(null);
  const [renamePath, setRenamePath] = useState(path);
  const [deleteTarget, setDeleteTarget] = useState<BrowseEntry | null>(null);
  const [deletePath, setDeletePath] = useState(path);
  const [mkdirOpen, setMkdirOpen] = useState(false);
  const [mkdirPath, setMkdirPath] = useState(path);
  const [nameInput, setNameInput] = useState("");
  const [busy, setBusy] = useState(false);
  // busy 状态要等渲染后才生效，回车连按两下会重复提交，用 ref 做同步锁。
  const busyRef = useRef(false);
  const renamedRef = useRef<RenamedHandler | null>(null);
  const deletedRef = useRef<DeletedHandler | null>(null);

  const run = useCallback(async (task: () => Promise<void>) => {
    if (busyRef.current) return;
    busyRef.current = true;
    setBusy(true);
    try {
      await task();
    } finally {
      busyRef.current = false;
      setBusy(false);
    }
  }, []);

  const settle = useCallback(
    async (at: string, until: (list: BrowseEntry[]) => boolean) => {
      const result = await cache.refresh(at, until);
      if (!result.settled && !result.error) {
        notify("列表还没刷出这次改动，稍后手动刷新看看", "warn");
      }
    },
    [cache, notify],
  );

  const openRename = (
    entry: BrowseEntry,
    at: string = path,
    onRenamed?: RenamedHandler,
  ) => {
    setNameInput(entry.name);
    setRenamePath(at);
    setRenameTarget(entry);
    renamedRef.current = onRenamed ?? null;
  };

  const closeRename = () => {
    renamedRef.current = null;
    setRenameTarget(null);
  };

  const openDelete = (
    entry: BrowseEntry,
    at: string = path,
    onDeleted?: DeletedHandler,
  ) => {
    setDeletePath(at);
    setDeleteTarget(entry);
    deletedRef.current = onDeleted ?? null;
  };

  const closeDelete = () => {
    deletedRef.current = null;
    setDeleteTarget(null);
  };

  const openMkdir = (at: string = path) => {
    setNameInput("");
    setMkdirPath(at);
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
      closeRename();
      return;
    }
    await run(async () => {
      const target = renameTarget;
      const at = renamePath;
      const done = renamedRef.current;
      const from = joinPath(at, target.name);
      const to = joinPath(at, name);
      try {
        await renameFile(from, name);
        notify(`已重命名为 ${name}`, "ok");
        // 请求期间用户可能关掉又打开了别的条目，只关掉自己那一次。
        setRenameTarget((current) => (current === target ? null : current));
        cache.invalidateSubtree(from);
        cache.invalidateSubtree(to);
        await settle(at, (list) => list.some((entry) => entry.name === name));
        done?.(from, to);
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "重命名失败", "error");
      }
    });
  }, [
    cache,
    nameInput,
    notify,
    renamePath,
    renameTarget,
    run,
    settle,
  ]);

  const submitDelete = useCallback(async () => {
    if (!deleteTarget) return;
    await run(async () => {
      const target = deleteTarget;
      const at = deletePath;
      const done = deletedRef.current;
      const removed = joinPath(at, target.name);
      try {
        await deleteFile(removed);
        notify(`已移入回收站：${target.name}`, "ok");
        setDeleteTarget((current) => (current === target ? null : current));
        cache.invalidateSubtree(removed);
        await settle(at, (list) => !list.some((entry) => entry.fid === target.fid));
        done?.(removed);
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "删除失败", "error");
      }
    });
  }, [cache, deletePath, deleteTarget, notify, run, settle]);

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
        await createFolder(joinPath(mkdirPath, name));
        notify(`已新建文件夹 ${name}`, "ok");
        setMkdirOpen(false);
        await settle(mkdirPath, (list) =>
          list.some((entry) => entry.name === name),
        );
      } catch (cause) {
        notify(cause instanceof Error ? cause.message : "新建文件夹失败", "error");
      }
    });
  }, [mkdirPath, nameInput, notify, run, settle]);

  return {
    renameTarget,
    renamePath,
    openRename,
    closeRename,
    deleteTarget,
    openDelete,
    closeDelete,
    mkdirOpen,
    mkdirPath,
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
