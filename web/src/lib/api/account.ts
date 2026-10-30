import { post, request } from "./client";

export type QuarkQR = { id: string; qr_url: string; expires_in: number };

export function startQuarkQR() {
  return post<QuarkQR>("/api/quark/qr", {});
}

export type QuarkQRStatus = {
  status: "waiting" | "confirmed" | "expired" | "error";
  nickname?: string;
  cookies?: string[];
  message?: string;
};

export function pollQuarkQR(id: string) {
  return request<QuarkQRStatus>(`/api/quark/qr/${encodeURIComponent(id)}`);
}

export type QuarkAccount = {
  ok: boolean;
  nickname?: string;
  checked_at?: string;
  message?: string;
};

export function getQuarkAccount() {
  return request<QuarkAccount>("/api/quark/account");
}
