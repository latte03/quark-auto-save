export const categoryLabels: Record<string, string> = {
  video: "视频",
  image: "图片",
  audio: "音频",
  doc: "文档",
  archive: "压缩包",
};

export function joinPath(base: string, name: string) {
  return base === "/" ? `/${name}` : `${base}/${name}`;
}

export function parentPath(path: string) {
  const index = path.lastIndexOf("/");
  return index <= 0 ? "/" : path.slice(0, index);
}
