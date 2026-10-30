import type { Task, TasksPayload } from "../types";
import { post, request } from "./client";

export function getTasks() {
  return request<TasksPayload>("/api/tasks");
}

export function createTask(task: Task) {
  return post<{ tasks: Task[] }>("/api/tasks", { task });
}

export function updateTask(index: number, task: Task) {
  return request<{ tasks: Task[] }>(`/api/tasks/${index}`, {
    method: "PUT",
    body: JSON.stringify({ task }),
  });
}

export function deleteTask(index: number) {
  return request<{ tasks: Task[] }>(`/api/tasks/${index}`, {
    method: "DELETE",
  });
}
