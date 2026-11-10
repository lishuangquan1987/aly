import type { AlyOutput } from "./models";

/**
 * 所有命令的统一响应。data 为 null 表示无载荷或下载完成。
 * 业务失败不会 throw：isSuccess=false 时 errorMsg 说明原因。
 */
export type AlyResponse<T = null> = Omit<AlyOutput, "data"> & { data: T };

/** 构造成功响应。 */
export function ok<T = null>(data: T): AlyResponse<T> {
  return { isSuccess: true, errorMsg: "", data };
}

/** 构造失败响应；失败响应没有 data。 */
export function ng<T = null>(errorMsg: string): AlyResponse<T> {
  return { isSuccess: false, errorMsg, data: null as unknown as T };
}