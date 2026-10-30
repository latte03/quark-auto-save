import { createContext, useContext, type ReactNode } from "react";
import type { ToastTone } from "./Toast";

export type Notify = (message: string, tone?: ToastTone) => void;

const NotifyContext = createContext<Notify>(() => {});

export function ToastProvider({
  notify,
  children,
}: {
  notify: Notify;
  children: ReactNode;
}) {
  return <NotifyContext.Provider value={notify}>{children}</NotifyContext.Provider>;
}

export function useToast() {
  return useContext(NotifyContext);
}
