import type { Transition } from "motion/react"

/** Mirrors --ease-out in index.css; built-in CSS curves feel weak in UI. */
export const EASE_OUT = [0.23, 1, 0.32, 1] as const

/** One surface growing out of another: the trigger and its panel. */
export const MORPH: Transition = { type: "spring", duration: 0.5, bounce: 0.22 }

/** Shared-layout glides, such as a pill travelling between tabs. */
export const SPRING_LAYOUT: Transition = {
  type: "spring",
  stiffness: 360,
  damping: 32,
  mass: 0.6,
}

/** Slots trading places inside a control, such as a label and its icon. */
export const SPRING_SWAP: Transition = {
  type: "spring",
  stiffness: 460,
  damping: 30,
  mass: 0.55,
}
