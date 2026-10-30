import { PageHeader } from "../components/PageHeader";
import { DownloadQueue } from "./files/DownloadQueue";
import { FileBrowser } from "./files/FileBrowser";

export function FilesPage() {
  return (
    <div className="space-y-4">
      <PageHeader
        title="文件"
        description="夸克网盘目录与本地下载队列。"
      />
      <DownloadQueue />
      <FileBrowser />
    </div>
  );
}
