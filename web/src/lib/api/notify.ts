import { post } from "./client";

export type NotifyTestChannel = {
  channel: string;
  ok: boolean;
  error?: string;
};

export function testNotify() {
  return post<{ channels: NotifyTestChannel[]; log: string }>(
    "/api/notify/test",
    {},
  );
}
