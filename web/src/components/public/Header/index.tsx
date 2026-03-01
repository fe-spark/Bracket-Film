"use client";

import React, { useState, useEffect, useCallback, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Input, Button, Popover, Empty } from "antd";
import {
  SearchOutlined,
  HistoryOutlined,
  DeleteOutlined,
} from "@ant-design/icons";
import { ApiGet } from "@/lib/api";
import { cookieUtil, COOKIE_KEY_MAP } from "@/lib/cookie";
import styles from "./index.module.less";
import { useAppMessage } from "@/lib/useAppMessage";

interface NavItem {
  id: string;
  name: string;
}

interface HistoryItem {
  id: string;
  name: string;
  episode: string;
  link: string;
  timeStamp: number;
}

export default function Header() {
  const [keyword, setKeyword] = useState("");
  const [navList, setNavList] = useState<NavItem[]>([]);
  const [siteInfo, setSiteInfo] = useState<any>({});
  const [historyList, setHistoryList] = useState<HistoryItem[]>([]);
  const [scrolled, setScrolled] = useState(false);
  const router = useRouter();
  const searchParams = useSearchParams();
  const { message } = useAppMessage();

  const urlSearch = searchParams.get("search") || "";
  const [prevUrlSearch, setPrevUrlSearch] = useState(urlSearch);
  if (prevUrlSearch !== urlSearch) {
    setPrevUrlSearch(urlSearch);
    setKeyword(urlSearch);
  }

  useEffect(() => {
    const handleScroll = () => {
      const scrollY = window.scrollY || document.documentElement.scrollTop;
      setScrolled(scrollY > 20);
    };
    window.addEventListener("scroll", handleScroll);
    return () => window.removeEventListener("scroll", handleScroll);
  }, []);

  useEffect(() => {
    ApiGet("/navCategory").then((resp) => {
      if (resp.code === 0) setNavList(resp.data || []);
    });
    ApiGet("/config/basic").then((resp) => {
      if (resp.code === 0) setSiteInfo(resp.data || {});
    });
  }, []);

  const loadHistory = useCallback(() => {
    const raw = cookieUtil.getCookie(COOKIE_KEY_MAP.FILM_HISTORY);
    if (raw) {
      try {
        const historyMap = JSON.parse(raw);
        const list = Object.values(historyMap) as HistoryItem[];
        list.sort((a, b) => b.timeStamp - a.timeStamp);
        setHistoryList(list);
      } catch (e) {
        setHistoryList([]);
      }
    } else {
      setHistoryList([]);
    }
  }, []);

  const handleClearHistory = (e: React.MouseEvent) => {
    e.stopPropagation();
    cookieUtil.clearCookie(COOKIE_KEY_MAP.FILM_HISTORY);
    setHistoryList([]);
    message.success("已清空历史记录");
  };

  const handleSearch = () => {
    if (!keyword.trim()) {
      message.error("请输入搜索关键词");
      return;
    }
    router.push(`/search?search=${encodeURIComponent(keyword)}`);
  };

  const [showHistory, setShowHistory] = useState(false);
  const historyRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (historyRef.current && !historyRef.current.contains(event.target as Node)) {
        setShowHistory(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  useEffect(() => {
    if (showHistory) {
      loadHistory();
    }
  }, [showHistory, loadHistory]);

  const historyContent = (
    <div className={`${styles.historyPanel} ${showHistory ? styles.show : ""}`}>
      <div className={styles.historyHeader}>
        <HistoryOutlined className={styles.icon} />
        <span className={styles.title}>历史观看记录</span>
        {historyList.length > 0 && (
          <DeleteOutlined
            className={styles.clear}
            onClick={handleClearHistory}
          />
        )}
      </div>
      <div className={styles.historyList}>
        {historyList.length > 0 ? (
          historyList.map((item, idx) => (
            <div
              key={idx}
              className={styles.historyItem}
              onClick={() => {
                router.push(item.link);
                setShowHistory(false);
              }}
              style={{ cursor: "pointer" }}
            >
              <span className={styles.filmTitle}>{item.name}</span>
              <span className={styles.episode}>{item.episode}</span>
            </div>
          ))
        ) : (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="暂无观看记录"
          />
        )}
      </div>
    </div>
  );

  return (
    <div className={`${styles.headerWrap} ${scrolled ? styles.scrolled : ""}`}>
      <div className={styles.headerInner}>
        <div className={styles.left}>
          {siteInfo.siteName && (
            <div className={styles.siteName} onClick={() => router.push("/")}>
              {siteInfo.siteName}
            </div>
          )}
          <div className={styles.searchGroup}>
            <Input
              placeholder="搜索影片、动漫、剧集..."
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSearch()}
              variant="borderless"
            />
            <Button
              type="primary"
              icon={<SearchOutlined />}
              onClick={handleSearch}
              className={styles.searchBtn}
            >
              搜索
            </Button>
          </div>
        </div>

        <div className={styles.right}>
          <div className={styles.navLinks}>
            <a onClick={() => router.push("/")} style={{ cursor: "pointer" }}>
              首页
            </a>
            {navList.map((nav) => (
              <a
                key={nav.id}
                onClick={() => router.push(`/filmClassify?Pid=${nav.id}`)}
                style={{ cursor: "pointer" }}
              >
                {nav.name}
              </a>
            ))}
          </div>

          <div className={styles.historyWrapper} ref={historyRef}>
            <div className={styles.historyBtn} onClick={() => setShowHistory(!showHistory)}>
              <HistoryOutlined />
            </div>
            {historyContent}
          </div>

          <div
            className={styles.mobileSearchBtn}
            onClick={() => router.push("/search")}
          >
            <SearchOutlined />
          </div>
        </div>
      </div>
    </div>
  );
}
