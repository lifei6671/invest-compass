import { invoke } from "@tauri-apps/api/core";

type CoreEnvelope<T> = {
  code: number;
  message: string;
  data: T;
  traceId?: string;
  requestId?: string;
};

export type CoreHealth = {
  status: string;
  version: string;
};

function unwrapCoreResponse<T>(response: CoreEnvelope<T>): T {
  if (response.code !== 0) {
    throw new Error(response.message || "本地核心服务调用失败");
  }
  return response.data;
}

/// 通过固定 Rust command 读取 Go core 健康状态，前端不接触 Go core 地址或 token。
export async function coreHealth(): Promise<CoreHealth> {
  const response = await invoke<CoreEnvelope<CoreHealth>>("core_health");
  return unwrapCoreResponse(response);
}
