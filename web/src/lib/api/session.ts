import { post, request } from "./client";

export type SessionInfo = {
  authenticated: boolean;
  username: string;
  version: string;
};

export function getSession() {
  return request<SessionInfo>("/api/session");
}

export type LoginResult = { token: string; username: string };

export function login(username: string, password: string) {
  return post<LoginResult>("/api/login", { username, password });
}

export function logout() {
  return post<{ success: boolean }>("/api/logout", {});
}
