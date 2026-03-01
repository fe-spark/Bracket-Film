"use client";

import axios, {
  AxiosInstance,
  AxiosRequestConfig,
  InternalAxiosRequestConfig,
} from "axios";
import { getToken, setToken } from "./auth";
import { message } from "antd";

const instance: AxiosInstance = axios.create({
  baseURL: "/api",
  timeout: 80000,
});



// 请求拦截器
instance.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = getToken();
    if (token && token.value.length > 0) {
      config.headers[token.key] = token.value;
    }
    return config;
  },
  (error) => Promise.reject(error),
);

// 响应拦截器
instance.interceptors.response.use(
  (response) => {
    const newToken = response.headers["new-token"];
    if (newToken && newToken.length > 0) {
      setToken(newToken);
    }
    return response.data;
  },
  (error) => {
    if (error.response?.status === 401) {
      message.error(error.response.data?.msg || "请先登录");
      window.location.href = "/login";
    } else if (error.response?.status === 403) {
      message.error("无访问权限");
    } else {
      message.error("服务器繁忙，请稍后再试");
    }
    return Promise.reject(error);
  },
);

// 通用响应类型
export interface ApiResponse<T = any> {
  code: number;
  msg: string;
  data: T;
}

export const ApiGet = <T = any>(
  url: string,
  params?: Record<string, any>,
): Promise<ApiResponse<T>> => {
  return instance.get(url, { params }) as any;
};

export const ApiPost = <T = any>(
  url: string,
  data?: any,
): Promise<ApiResponse<T>> => {
  return instance.post(url, data) as any;
};

export default instance;
