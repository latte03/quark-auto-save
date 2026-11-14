import { useEffect, useRef } from "react";
import type { Page } from "../../lib/types";

/**
 * 侧边栏真·path 路径动画图标：每个图标由若干参数化形状组成，激活时
 * rAF 逐帧插值形状参数并重新生成 `d`——轮廓本身伸缩/位移（描边粗细
 * 恒定），而不是整体 transform 缩放。`prefers-reduced-motion` 下不播。
 */

export type MenuIconId = Extract<
  Page,
  "overview" | "tasks" | "files" | "runs" | "settings"
>;

type Params = Record<string, number>;

type Keyframe = { at: number; p: Params; easing?: string };

type Shape = {
  render: (p: Params) => string;
  base: Params;
  kfs: Keyframe[];
};

const DURATION = 720;

const kf = (at: number, p: Params, easing?: string): Keyframe => ({
  at,
  p,
  ...(easing ? { easing } : {}),
});

/* WAAPI 用字符串 easing（动画由浏览器时钟驱动，不依赖 rAF——
   部分 webview 环境会冻结 rAF 但照常播放 CSS/WAAPI 动画）。 */
const EASE_IN_OUT = "cubic-bezier(0.65, 0, 0.35, 1)";
const EASE_OUT_BACK = "cubic-bezier(0.34, 1.56, 0.64, 1)";

/** 圆角矩形（闭合，参数 x/y/w/h/r），r 自动钳制防止压扁时翻转。 */
function rectPath(p: Params) {
  const { x, y, w, h } = p;
  const r = Math.max(0, Math.min(p.r, w / 2, h / 2));
  return (
    `M ${x + r} ${y} L ${x + w - r} ${y} A ${r} ${r} 0 0 1 ${x + w} ${y + r} ` +
    `L ${x + w} ${y + h - r} A ${r} ${r} 0 0 1 ${x + w - r} ${y + h} ` +
    `L ${x + r} ${y + h} A ${r} ${r} 0 0 1 ${x} ${y + h - r} ` +
    `L ${x} ${y + r} A ${r} ${r} 0 0 1 ${x + r} ${y} Z`
  );
}

/** 水平线（开放路径）。 */
function linePath(p: Params) {
  return `M ${p.x1} ${p.y} L ${p.x2} ${p.y}`;
}

/** 圆（两段圆弧闭合），cx/cy 可动画 → 圆在轨道上真实滑动。 */
function circlePath(p: Params) {
  const { cx, cy } = p;
  const r = p.r;
  return (
    `M ${cx - r} ${cy} a ${r} ${r} 0 1 0 ${2 * r} 0 ` +
    `a ${r} ${r} 0 1 0 ${-2 * r} 0`
  );
}

/** 静态形状（d 不随动画变化）。 */
function staticPath(d: string): Shape & { delay: number } {
  return {
    render: () => d,
    base: {},
    kfs: [kf(0, {}), kf(DURATION, {})],
    delay: 0,
  };
}

/**
 * 方块的"旅程"：拉长下移 → 压扁右移 → 拉长上移 → 压扁左移 → 回弹归位。
 * 幅度（my/mx/st/sq）每块不同，位置与尺寸逐帧写死为绝对参数。
 */
function blockJourney(
  b: Params,
  amp: { my: number; mx: number; st: number; sq: number },
): Keyframe[] {
  // r 必须逐帧带上：rectPath 的圆角钳制遇 undefined 会产出 NaN，
  // 一帧 NaN 就让整条 @keyframes 被浏览器丢弃。
  const { x, y, w, h, r } = b;
  const wide = w * 1.1;
  const tall = h * amp.st;
  const flat = h * amp.sq;
  return [
    kf(0, b),
    kf(110, { x, y: y + amp.my, w, h: tall, r }),
    kf(230, { x: x + amp.mx, y: y + amp.my, w: wide, h: flat, r }),
    kf(350, { x: x + amp.mx, y: y - amp.my, w: wide, h: tall, r }),
    kf(470, { x: x - amp.mx, y: y - amp.my, w: wide, h: flat, r }, EASE_OUT_BACK),
    kf(590, { x: x - amp.mx, y: y - amp.my * 0.3, w, h: h * 1.06, r }),
    kf(DURATION, b),
  ];
}

const ICON_DEFS: Record<
  MenuIconId,
  { shapes: Array<Shape & { delay: number }>; viewBox: string }
> = {
  // 总览：lucide layout-dashboard 的四方块，逐块游走接力。
  // 幅度按各块在 24 视野内的边界校过（拉长/位移明显的才是"真在动"）。
  overview: {
    viewBox: "0 0 24 24",
    shapes: [
      {
        render: rectPath,
        base: { x: 3, y: 3, w: 7, h: 9, r: 1 },
        kfs: blockJourney(
          { x: 3, y: 3, w: 7, h: 9, r: 1 },
          { my: 2.2, mx: 2.4, st: 1.35, sq: 0.55 },
        ),
        delay: 0,
      },
      {
        render: rectPath,
        base: { x: 14, y: 3, w: 7, h: 5, r: 1 },
        kfs: blockJourney(
          { x: 14, y: 3, w: 7, h: 5, r: 1 },
          { my: 1.6, mx: 1.8, st: 1.3, sq: 0.6 },
        ),
        delay: 80,
      },
      {
        render: rectPath,
        base: { x: 14, y: 12, w: 7, h: 9, r: 1 },
        kfs: blockJourney(
          { x: 14, y: 12, w: 7, h: 9, r: 1 },
          { my: 0.8, mx: 1.6, st: 1.2, sq: 0.55 },
        ),
        delay: 160,
      },
      {
        render: rectPath,
        base: { x: 3, y: 16, w: 7, h: 5, r: 1 },
        kfs: blockJourney(
          { x: 3, y: 16, w: 7, h: 5, r: 1 },
          { my: 1.2, mx: 1.4, st: 1.3, sq: 0.55 },
        ),
        delay: 240,
      },
    ],
  },

  // 任务：lucide list-video 的三条列表线 + 媒体块；线像列表刷新一样伸缩。
  tasks: {
    viewBox: "0 0 24 24",
    shapes: [
      {
        render: linePath,
        base: { x1: 3, x2: 21, y: 5 },
        kfs: [
          kf(0, { x1: 3, x2: 21, y: 5 }),
          kf(130, { x1: 3, x2: 14.5, y: 5 }),
          kf(380, { x1: 3, x2: 21.6, y: 5 }, EASE_OUT_BACK),
          kf(560, { x1: 3, x2: 20.6, y: 5 }),
          kf(DURATION, { x1: 3, x2: 21, y: 5 }),
        ],
        delay: 0,
      },
      {
        render: linePath,
        base: { x1: 3, x2: 10, y: 12 },
        kfs: [
          kf(0, { x1: 3, x2: 10, y: 12 }),
          kf(190, { x1: 3, x2: 14, y: 12 }, EASE_OUT_BACK),
          kf(440, { x1: 3, x2: 9.2, y: 12 }),
          kf(600, { x1: 3, x2: 10.2, y: 12 }),
          kf(DURATION, { x1: 3, x2: 10, y: 12 }),
        ],
        delay: 0,
      },
      {
        render: linePath,
        base: { x1: 3, x2: 10, y: 19 },
        kfs: [
          kf(0, { x1: 3, x2: 10, y: 19 }),
          kf(190, { x1: 3, x2: 14, y: 19 }, EASE_OUT_BACK),
          kf(440, { x1: 3, x2: 9.2, y: 19 }),
          kf(600, { x1: 3, x2: 10.2, y: 19 }),
          kf(DURATION, { x1: 3, x2: 10, y: 19 }),
        ],
        delay: 60,
      },
      {
        render: rectPath,
        base: { x: 15, y: 12, w: 6.5, h: 6.5, r: 1.2 },
        kfs: blockJourney(
          { x: 15, y: 12, w: 6.5, h: 6.5, r: 1.2 },
          { my: 0.9, mx: 1, st: 1.14, sq: 0.76 },
        ),
        delay: 120,
      },
    ],
  },

  // 文件：文件夹 tab 下压实合 → 上掀展开，body 顶部联动压扁/拉伸。
  files: {
    viewBox: "0 0 24 24",
    shapes: [
      {
        render: rectPath,
        base: { x: 3, y: 4, w: 8, h: 4.5, r: 1 },
        kfs: [
          kf(0, { x: 3, y: 4, w: 8, h: 4.5, r: 1 }),
          kf(130, { x: 3, y: 6.2, w: 9.2, h: 5, r: 1 }),
          kf(380, { x: 3, y: 3.2, w: 9.6, h: 4.2, r: 1 }, EASE_OUT_BACK),
          kf(560, { x: 3, y: 4.4, w: 8.2, h: 4.5, r: 1 }),
          kf(DURATION, { x: 3, y: 4, w: 8, h: 4.5, r: 1 }),
        ],
        delay: 0,
      },
      {
        render: rectPath,
        base: { x: 3, y: 8, w: 18, h: 12, r: 1.6 },
        kfs: [
          kf(0, { x: 3, y: 8, w: 18, h: 12, r: 1.6 }),
          kf(150, { x: 3, y: 9.8, w: 18, h: 10.2, r: 1.6 }),
          kf(400, { x: 3, y: 7.4, w: 18, h: 12.6, r: 1.6 }, EASE_OUT_BACK),
          kf(580, { x: 3, y: 7.8, w: 18, h: 12.2, r: 1.6 }),
          kf(DURATION, { x: 3, y: 8, w: 18, h: 12, r: 1.6 }),
        ],
        delay: 0,
      },
    ],
  },

  // 运行记录：保留 lucide scroll-text 卷轴轮廓，两条文字线真实伸缩。
  runs: {
    viewBox: "0 0 24 24",
    shapes: [
      staticPath(
        "M8 21h12a2 2 0 0 0 2-2v-1a1 1 0 0 0-1-1H11a1 1 0 0 0-1 1v1a2 2 0 1 1-4 0V5a2 2 0 1 0-4 0v2a1 1 0 0 0 1 1h3",
      ),
      staticPath("M19 17V5a2 2 0 0 0-2-2H4"),
      {
        render: linePath,
        base: { x1: 10, x2: 15, y: 8 },
        kfs: [
          kf(0, { x1: 10, x2: 15, y: 8 }),
          kf(130, { x1: 13, x2: 15, y: 8 }),
          kf(380, { x1: 8.6, x2: 16.4, y: 8 }, EASE_OUT_BACK),
          kf(560, { x1: 10.3, x2: 15.2, y: 8 }),
          kf(DURATION, { x1: 10, x2: 15, y: 8 }),
        ],
        delay: 0,
      },
      {
        render: linePath,
        base: { x1: 10, x2: 15, y: 12 },
        kfs: [
          kf(0, { x1: 10, x2: 15, y: 12 }),
          kf(190, { x1: 7.8, x2: 16.2, y: 12 }, EASE_OUT_BACK),
          kf(440, { x1: 12.8, x2: 13.8, y: 12 }),
          kf(600, { x1: 9.7, x2: 15.1, y: 12 }),
          kf(DURATION, { x1: 10, x2: 15, y: 12 }),
        ],
        delay: 0,
      },
    ],
  },

  // 设置：两个滑钮沿轨道真实滑动（圆弧 path 平移），线端点联动伸缩。
  settings: {
    viewBox: "0 0 24 24",
    shapes: [
      {
        render: linePath,
        base: { x1: 5, x2: 14, y: 17 },
        kfs: [
          kf(0, { x1: 5, x2: 14, y: 17 }),
          kf(160, { x1: 5, x2: 9.3, y: 17 }),
          kf(380, { x1: 5, x2: 16.1, y: 17 }),
          kf(560, { x1: 5, x2: 13.1, y: 17 }),
          kf(DURATION, { x1: 5, x2: 14, y: 17 }),
        ],
        delay: 0,
      },
      {
        render: circlePath,
        base: { cx: 17, cy: 17, r: 3 },
        kfs: [
          kf(0, { cx: 17, cy: 17, r: 3 }),
          kf(160, { cx: 12.8, cy: 17, r: 3 }),
          kf(380, { cx: 19.6, cy: 17, r: 3 }, EASE_OUT_BACK),
          kf(560, { cx: 16.6, cy: 17, r: 3 }),
          kf(DURATION, { cx: 17, cy: 17, r: 3 }),
        ],
        delay: 0,
      },
      {
        render: linePath,
        base: { x1: 10, x2: 19, y: 7 },
        kfs: [
          kf(0, { x1: 10, x2: 19, y: 7 }),
          kf(160, { x1: 13.9, x2: 19, y: 7 }),
          kf(380, { x1: 7.7, x2: 19, y: 7 }),
          kf(560, { x1: 10.9, x2: 19, y: 7 }),
          kf(DURATION, { x1: 10, x2: 19, y: 7 }),
        ],
        delay: 140,
      },
      {
        render: circlePath,
        base: { cx: 7, cy: 7, r: 3 },
        kfs: [
          kf(0, { cx: 7, cy: 7, r: 3 }),
          kf(160, { cx: 10.4, cy: 7, r: 3 }),
          kf(380, { cx: 4.2, cy: 7, r: 3 }, EASE_OUT_BACK),
          kf(560, { cx: 7.4, cy: 7, r: 3 }),
          kf(DURATION, { cx: 7, cy: 7, r: 3 }),
        ],
        delay: 140,
      },
    ],
  },
};

const iconStyleId = (id: MenuIconId) => `qas-icon-${id}`;

/** 把图标的每个形状编译成 @keyframes（d 的 CSS 帧全部用 path("...")），
    首次使用时注入一次。CSS 动画由浏览器时钟驱动，在会冻结 rAF 的
    webview 里依然照常播放。 */
function ensureIconStyles(id: MenuIconId, def: (typeof ICON_DEFS)[MenuIconId]) {
  const styleId = iconStyleId(id);
  if (document.getElementById(styleId)) return;
  const css = def.shapes
    .map((shape, i) => {
      const frames = shape.kfs
        .map((k) => {
          const timing = `animation-timing-function: ${k.easing ?? EASE_IN_OUT};`;
          return `${((k.at / DURATION) * 100).toFixed(1)}% { d: path("${shape.render(k.p)}"); ${timing} }`;
        })
        .join("\n");
      return `@keyframes ${styleId}-${i} {\n${frames}\n}`;
    })
    .join("\n");
  const style = document.createElement("style");
  style.id = styleId;
  style.textContent = css;
  document.head.appendChild(style);
}

export function MenuIcon({
  id,
  active,
  className,
}: {
  id: MenuIconId;
  active: boolean;
  className?: string;
}) {
  const def = ICON_DEFS[id];
  const refs = useRef<Array<SVGPathElement | null>>([]);

  useEffect(() => {
    if (!active) return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;

    // 激活时给每个形状挂 CSS d 动画（delay 做接力），失活时摘掉——
    // path 回到 React 渲染的 base d，中断不残留变形帧。
    ensureIconStyles(id, def);
    const styleId = iconStyleId(id);
    def.shapes.forEach((_, i) => {
      const el = refs.current[i];
      if (!el) return;
      el.style.animation = `${styleId}-${i} ${DURATION}ms both ${def.shapes[i].delay}ms`;
    });
    return () => {
      def.shapes.forEach((_, i) => {
        refs.current[i]?.style.setProperty("animation", "none");
      });
    };
  }, [active, def]);

  return (
    <svg
      viewBox={def.viewBox}
      fill="none"
      stroke="currentColor"
      strokeWidth={1.6}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden
      className={className}
    >
      {def.shapes.map((shape, i) => (
        <path
          key={i}
          ref={(el) => {
            refs.current[i] = el;
          }}
          d={shape.render(shape.base)}
        />
      ))}
    </svg>
  );
}
