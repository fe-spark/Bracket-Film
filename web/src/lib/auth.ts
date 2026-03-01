// Token 管理

interface AuthToken {
  key: string;
  value: string;
}

export const setToken = (token: string) => {
  const auth: AuthToken = { key: "auth-token", value: token };
  localStorage.setItem("auth", JSON.stringify(auth));
};

export const getToken = (): AuthToken | null => {
  if (typeof window === "undefined") return null;
  const raw = localStorage.getItem("auth");
  if (!raw) return null;
  try {
    return JSON.parse(raw) as AuthToken;
  } catch {
    return null;
  }
};

export const clearAuthToken = () => {
  localStorage.removeItem("auth");
};
