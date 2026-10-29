import { TrimApp } from "@trimjs/web-app";

let instance: TrimApp | null = null;

function sdk() {
  if (!instance) instance = new TrimApp();
  return instance;
}

export async function pickSharedDirectory(): Promise<string | null> {
  const app = sdk();
  await app.ready();
  if (app.isStandaloneWeb) {
    throw new Error("请在飞牛桌面中打开 QAS 选择目录，或手动填写路径");
  }
  const result = await app.pickSharedFile({
    title: "选择下载目录",
    okText: "授权并使用",
    sidebarGroup: ["myFiles", "external", "remote"],
  });
  if (!result) return null;
  if (result.code !== 0) {
    throw new Error(result.msg || "目录选择失败");
  }
  return result.data?.[0] ?? null;
}
