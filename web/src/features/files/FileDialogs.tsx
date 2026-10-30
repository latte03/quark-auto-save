import { Loader2 } from "lucide-react";
import { Button } from "../../components/ui/button";
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "../../components/ui/dialog";
import { Input } from "../../components/ui/input";
import { joinPath } from "./paths";
import type { FileActions } from "./useFileActions";

export function FileDialogs({
  path,
  actions,
}: {
  path: string;
  actions: FileActions;
}) {
  const {
    renameTarget,
    closeRename,
    deleteTarget,
    closeDelete,
    mkdirOpen,
    setMkdirOpen,
    nameInput,
    setNameInput,
    busy,
    submitRename,
    submitDelete,
    submitMkdir,
  } = actions;

  return (
    <>
      <Dialog open={Boolean(renameTarget)} onOpenChange={(open) => !open && closeRename()}>
        <DialogContent className="w-[min(440px,calc(100vw-2rem))]">
          <DialogHeader>
            <DialogTitle>重命名</DialogTitle>
            <DialogDescription>
              当前路径：{renameTarget ? joinPath(path, renameTarget.name) : "—"}
            </DialogDescription>
          </DialogHeader>
          <DialogBody>
            <Input
              autoFocus
              value={nameInput}
              onChange={(event) => setNameInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void submitRename();
              }}
              aria-label="新名称"
            />
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" size="sm" onClick={closeRename}>
              取消
            </Button>
            <Button size="sm" disabled={busy} onClick={() => void submitRename()}>
              {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
              保存
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && closeDelete()}>
        <DialogContent className="w-[min(420px,calc(100vw-2rem))]">
          <DialogHeader>
            <DialogTitle>删除文件</DialogTitle>
            <DialogDescription>
              文件会移入夸克网盘回收站，可在网盘侧恢复。
            </DialogDescription>
          </DialogHeader>
          <DialogBody>
            <div className="rounded-md border border-danger/20 bg-danger-soft/40 px-3 py-2.5 text-[13px] text-ink">
              确定删除「{deleteTarget?.name}
              {deleteTarget?.is_dir ? "」（含目录内全部文件）" : "」"}？
            </div>
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" size="sm" onClick={closeDelete}>
              取消
            </Button>
            <Button
              variant="destructive"
              size="sm"
              disabled={busy}
              onClick={() => void submitDelete()}
            >
              {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
              删除
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={mkdirOpen} onOpenChange={setMkdirOpen}>
        <DialogContent className="w-[min(440px,calc(100vw-2rem))]">
          <DialogHeader>
            <DialogTitle>新建文件夹</DialogTitle>
            <DialogDescription>创建在 {path}</DialogDescription>
          </DialogHeader>
          <DialogBody>
            <Input
              autoFocus
              value={nameInput}
              placeholder="文件夹名称"
              onChange={(event) => setNameInput(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") void submitMkdir();
              }}
              aria-label="文件夹名称"
            />
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" size="sm" onClick={() => setMkdirOpen(false)}>
              取消
            </Button>
            <Button size="sm" disabled={busy} onClick={() => void submitMkdir()}>
              {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : null}
              创建
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
