"use client";

import React, { useEffect, useMemo, useState, useCallback } from "react";
import { App, ConfigProvider, theme } from "antd";
import styles from "./GlobalThemeProvider.module.less";

type ThemeMode = "dark" | "light" | "system";

const STORAGE_KEY = "app-theme";

function resolveEffective(mode: ThemeMode): "dark" | "light" {
  if (mode !== "system") return mode;
  if (typeof window === "undefined") return "dark";
  return window.matchMedia("(prefers-color-scheme: light)").matches
    ? "light"
    : "dark";
}

function getSavedMode(): ThemeMode {
  if (typeof window === "undefined") return "system";
  const saved = localStorage.getItem(STORAGE_KEY);
  if (saved === "dark" || saved === "light" || saved === "system") return saved;
  return "system";
}

const ICON_SUN = (
  <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="5" />
    <line x1="12" y1="1" x2="12" y2="3" /><line x1="12" y1="21" x2="12" y2="23" />
    <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" /><line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
    <line x1="1" y1="12" x2="3" y2="12" /><line x1="21" y1="12" x2="23" y2="12" />
    <line x1="4.22" y1="19.78" x2="5.64" y2="18.36" /><line x1="18.36" y1="5.64" x2="19.78" y2="4.22" />
  </svg>
);
const ICON_MOON = (
  <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M21 12.79A9 9 0 1111.21 3 7 7 0 0021 12.79z" />
  </svg>
);
const ICON_SYSTEM = (
  <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <rect x="2" y="3" width="20" height="14" rx="2" ry="2" /><line x1="8" y1="21" x2="16" y2="21" /><line x1="12" y1="17" x2="12" y2="21" />
  </svg>
);

const OPTIONS: { key: ThemeMode; icon: React.ReactNode; label: string }[] = [
  { key: "light", icon: ICON_SUN, label: "浅色" },
  { key: "dark", icon: ICON_MOON, label: "深色" },
  { key: "system", icon: ICON_SYSTEM, label: "系统" },
];

export default function GlobalThemeProvider({
  children,
  fontFamily,
}: {
  children: React.ReactNode;
  fontFamily: string;
}) {
  // 为了避免服务端与客户端初始渲染不一致导致的 Hydration 报错，
  // 初始状态强制设为服务端默认值 ("system" / "dark")，然后在 useEffect 中同步本地存储
  const [mode, setMode] = useState<ThemeMode>("system");
  const [effective, setEffective] = useState<"dark" | "light">("dark");
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
    const saved = getSavedMode();
    setMode(saved);
    setEffective(resolveEffective(saved));
  }, []);

  const getCssVar = useCallback(
    (varName: string) => {
      if (typeof window === "undefined") return `var(${varName})`;
      // Directly check for the CSS variable name in computed properties
      const val = getComputedStyle(document.documentElement).getPropertyValue(varName).trim();
      return val || `var(${varName})`;
    },
    [effective],
  );

  // 监听系统主题变化（仅 system 模式下生效）
  useEffect(() => {
    const mql = window.matchMedia("(prefers-color-scheme: light)");
    const handler = () => {
      if (mode === "system") setEffective(resolveEffective("system"));
    };
    mql.addEventListener("change", handler);
    return () => mql.removeEventListener("change", handler);
  }, [mode]);

  useEffect(() => {
    setEffective(resolveEffective(mode));
    localStorage.setItem(STORAGE_KEY, mode);
  }, [mode]);

  useEffect(() => {
    document.documentElement.dataset.theme = effective;
    document.documentElement.style.colorScheme = effective;
  }, [effective]);

  const handleSelect = useCallback((m: ThemeMode) => {
    setMode(m);
  }, []);

  const isDark = effective === "dark";

  const providerTheme = useMemo(
    () => ({
      algorithm: isDark ? theme.darkAlgorithm : theme.defaultAlgorithm,
      token: {
        colorPrimary: getCssVar("--primary-color"),
        borderRadius: 12,
        fontFamily,
      },
      components: {
        Button: {
          controlHeightLG: 52,
          fontWeight: 600,
          borderRadiusLG: 14,
          boxShadow: "none",
          boxShadowSecondary: "none",
          boxShadowTertiary: "none",
        },
        Input: {
          controlHeight: 40,
          borderRadius: 10,
        },
        Popover: {
          colorBgElevated: getCssVar("--public-surface-2"),
          colorText: getCssVar("--public-text-2"),
          colorTextHeading: getCssVar("--public-text-1"),
        },
        Pagination: {
          itemSize: 50,
          itemBg: isDark ? "rgba(255, 255, 255, 0.06)" : "rgba(0, 0, 0, 0.04)",
          itemActiveBg: getCssVar("--primary-color"),
          itemActiveColor: "#ffffff",
          colorPrimary: getCssVar("--primary-color"),
          colorText: isDark ? "#ffffff" : "#101218",
          colorBgContainer: isDark ? "rgba(255, 255, 255, 0.03)" : "#ffffff",
          colorBorder: isDark ? "rgba(255, 255, 255, 0.12)" : "rgba(0, 0, 0, 0.1)",
          colorTextLightSolid: "#ffffff",
          colorTextDescription: isDark ? "rgba(255, 255, 255, 0.65)" : "rgba(16, 18, 24, 0.72)",
          colorTextDisabled: isDark ? "rgba(255, 255, 255, 0.45)" : "rgba(16, 18, 24, 0.44)",
          colorTextPlaceholder: isDark ? "rgba(255, 255, 255, 0.45)" : "rgba(16, 18, 24, 0.44)",
          colorTextTertiary: isDark ? "rgba(255, 255, 255, 0.45)" : "rgba(16, 18, 24, 0.44)",
        },
      },
    }),
    [fontFamily, isDark, getCssVar],
  );



  // 当前选中项的图标作为收起态按钮
  const activeOption = OPTIONS.find((o) => o.key === mode) || OPTIONS[2];

  return (
    <ConfigProvider theme={{ ...providerTheme, cssVar: { key: "app-theme" } }}>
      <App>
        {children}
        <div className={styles.floatingDock}>
          <div className={styles.indicator}>{activeOption.icon}</div>
          {OPTIONS.map((opt) => (
            <div
              key={opt.key}
              className={`${styles.option} ${mode === opt.key ? styles.active : ""}`}
              onClick={() => handleSelect(opt.key)}
            >
              {opt.icon}
              <span>{opt.label}</span>
            </div>
          ))}
        </div>
      </App>
    </ConfigProvider>
  );
}
