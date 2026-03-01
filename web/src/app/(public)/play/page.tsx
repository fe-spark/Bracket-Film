"use client";

import React, {
  useState,
  useEffect,
  useRef,
  Suspense,
  useCallback,
} from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "antd";
import {
  RocketOutlined,
  StepForwardOutlined,
  PlayCircleOutlined,
} from "@ant-design/icons";
import { ApiGet } from "@/lib/api";
import { cookieUtil, COOKIE_KEY_MAP } from "@/lib/cookie";
import FilmList from "@/components/public/FilmList";
import VideoPlayer from "@/components/public/VideoPlayer";
import AppLoading from "@/components/public/Loading";
import styles from "./page.module.less";
import { useAppMessage } from "@/lib/useAppMessage";

function PlayerContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const id = searchParams.get("id");
  const sourceId = searchParams.get("source");
  const episodeIdx = searchParams.get("episode");
  const initialTime = searchParams.get("currentTime");
  const { message } = useAppMessage();

  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [currentTabId, setCurrentTabId] = useState("");
  const [current, setCurrent] = useState<any>(null);
  const [autoplay, setAutoplay] = useState(true);
  const [playerError, setPlayerError] = useState(false);

  const activeEpRef = useRef<HTMLDivElement>(null);
  const activeTabRef = useRef<HTMLDivElement>(null);
  const sourceTabsRef = useRef<HTMLDivElement>(null);
  const episodeListRef = useRef<HTMLDivElement>(null);
  const leftColumnRef = useRef<HTMLDivElement>(null);
  const sidebarRef = useRef<HTMLDivElement>(null);

  // 1. 数据加载与状态同步
  useEffect(() => {
    if (!id) return;

    const load = async () => {
      setLoading(true);
      try {
        const resp = await ApiGet("/filmPlayInfo", {
          id,
          playFrom: sourceId,
          episode: episodeIdx || 0,
        });
        if (resp.code === 0) {
          setData(resp.data);
          setCurrent({ index: resp.data.currentEpisode, ...resp.data.current });
          setCurrentTabId(resp.data.currentPlayFrom);
          setPlayerError(false); // Reset error state on new resource load
        } else {
          message.error(resp.msg);
        }
      } finally {
        setLoading(false);
      }
    };

    void load();
  }, [id, sourceId, episodeIdx, message]);

  // 计算衍生数据，减少冗余逻辑
  const detail = data?.detail;
  const relate = data?.relate;
  const currentSource = detail?.list.find((s: any) => s.id === currentTabId);
  const hasNext = currentSource && current && current.index < currentSource.linkList.length - 1;

  // 侧边栏高度同步
  useEffect(() => {
    const leftEl = leftColumnRef.current;
    const sideEl = sidebarRef.current;
    if (!leftEl || !sideEl) return;

    const syncHeight = () => {
      sideEl.style.height = `${leftEl.offsetHeight}px`;
    };

    syncHeight();
    const ro = new ResizeObserver(syncHeight);
    ro.observe(leftEl);
    return () => ro.disconnect();
  }, [data]);

  // 自动滚动定位：当前集数和播放源
  useEffect(() => {
    const scrollToTarget = (container: HTMLElement | null, target: HTMLElement | null, isHorizontal = false) => {
      if (!container || !target) return;
      container.scrollTo({
        [isHorizontal ? "left" : "top"]: (isHorizontal ? target.offsetLeft - container.offsetWidth / 2 + target.offsetWidth / 2 : target.offsetTop - container.offsetHeight / 2 + target.offsetHeight / 2),
        behavior: "smooth",
      });
    };

    if (current?.link) scrollToTarget(episodeListRef.current, activeEpRef.current);
    if (currentTabId) scrollToTarget(sourceTabsRef.current, activeTabRef.current, true);
  }, [current?.link, currentTabId]);

  const handlePlayNext = useCallback(() => {
    if (hasNext) {
      router.replace(`/play?id=${id}&source=${currentTabId}&episode=${current.index + 1}`);
    } else {
      message.info("已经是最后一集了");
    }
  }, [hasNext, id, currentTabId, current?.index, router, message]);

  // 2. 核心逻辑：播放进度保存
  const handleTimeUpdate = useCallback(
    (currentTime: number, duration: number) => {
      if (!detail || !current) return;
      const historyRaw = cookieUtil.getCookie(COOKIE_KEY_MAP.FILM_HISTORY);
      let historyMap: any = {};
      if (historyRaw) {
        try {
          historyMap = JSON.parse(historyRaw);
        } catch (e) { }
      }

      historyMap[detail.id] = {
        id: detail.id,
        name: detail.name,
        picture: detail.picture,
        sourceId: currentTabId,
        episodeIndex: current.index,
        sourceName: currentSource?.name || "默认源",
        episode: current.episode || "正在观看",
        timeStamp: Date.now(),
        link: `/play?id=${detail.id}&source=${currentTabId}&episode=${current.index}`,
        currentTime,
        duration,
      };

      cookieUtil.setCookie(COOKIE_KEY_MAP.FILM_HISTORY, JSON.stringify(historyMap));
    },
    [detail, current, currentTabId, currentSource],
  );

  if (loading) return <AppLoading text="正在加载播放资源..." />;
  if (!data) return null;

  return (
    <div className={styles.container}>
      <div className={styles.bgWrapper}>
        <img src={detail.picture} className={styles.bgPoster} alt="background" />
        <div className={styles.mask} />
      </div>

      <div className={styles.mainContent}>
        <div className={styles.leftColumn} ref={leftColumnRef}>
          <div className={styles.topInfoCard}>
            <div className={styles.leftSection}>
              <h1 className={styles.filmTitle}>
                <a onClick={() => router.push(`/filmDetail?link=${detail.id}`)} style={{ cursor: "pointer" }}>
                  {detail.name}
                </a>
              </h1>
              <div className={styles.meta}>
                <span className={styles.active}>{detail.descriptor.remarks}</span>
                <span>|</span>
                <span>{detail.descriptor.cName}</span>
                <span>|</span>
                <span>{detail.descriptor.year}</span>
                <span>|</span>
                <span>{detail.descriptor.area}</span>
              </div>
            </div>
            <div className={styles.rightSection}>
              <div className={styles.extraInfo}>
                <div className={styles.scoreLabel}>综合评分</div>
                <div className={styles.scoreValue}>
                  {detail.descriptor.score || "9.0"}
                  <span>分</span>
                </div>
              </div>
            </div>
          </div>

          <div className={`${styles.playerWrapper} ${playerError ? styles.isPlayerError : ""}`}>
            {current?.link && (
              <VideoPlayer
                key={current.link}
                src={current.link}
                initialTime={initialTime ? parseFloat(initialTime) : 0}
                autoplay={autoplay}
                onEnded={() => autoplay && handlePlayNext()}
                onTimeUpdate={handleTimeUpdate}
                onError={() => {
                  setPlayerError(true);
                  message.error("该视频源加载失败，请尝试切换播放源。");
                }}
              />
            )}
          </div>
        </div>

        <div className={styles.sidebar} ref={sidebarRef}>
          <div className={styles.sideHeader}>
            <div className={styles.title}>正在播放</div>
            <div className={styles.subtitle}>{detail.name} - {current?.episode}</div>
          </div>

          <div className={styles.sourceTabs} ref={sourceTabsRef}>
            {detail.list.map((item: any) => {
              const isActive = currentTabId === item.id;
              return (
                <div
                  key={item.id}
                  ref={isActive ? activeTabRef : null}
                  className={`${styles.tab} ${isActive ? styles.active : ""}`}
                  onClick={() => setCurrentTabId(item.id)}
                >
                  {item.name}
                </div>
              );
            })}
          </div>

          <div className={styles.episodeList} ref={episodeListRef}>
            {currentSource?.linkList.map((v: any, i: number) => {
              const isActive = current.link === v.link;
              return (
                <div
                  key={i}
                  ref={isActive ? activeEpRef : undefined}
                  className={`${styles.epItem} ${isActive ? styles.active : ""}`}
                  title={v.episode}
                  onClick={() => router.replace(`/play?id=${id}&source=${currentTabId}&episode=${i}`)}
                  onMouseEnter={(e) => {
                    const span = e.currentTarget.querySelector<HTMLSpanElement>(`.${styles.epText}`);
                    if (span && span.scrollWidth > span.clientWidth) {
                      const overflow = span.scrollWidth - span.clientWidth;
                      const duration = overflow / 50 / 0.6;
                      span.style.setProperty("--scroll-distance", `-${overflow}px`);
                      span.style.setProperty("--scroll-duration", `${duration.toFixed(2)}s`);
                      span.classList.add(styles.marquee);
                    }
                  }}
                  onMouseLeave={(e) => {
                    const span = e.currentTarget.querySelector<HTMLSpanElement>(`.${styles.epText}`);
                    if (span) {
                      span.classList.remove(styles.marquee);
                      span.style.removeProperty("--scroll-distance");
                      span.style.removeProperty("--scroll-duration");
                    }
                  }}
                >
                  <span className={styles.epText}>{v.episode}</span>
                </div>
              );
            })}
          </div>

          <div className={styles.sideFooter}>
            <div
              className={`${styles.footerBtn} ${autoplay ? styles.active : ""}`}
              onClick={() => setAutoplay(!autoplay)}
            >
              <PlayCircleOutlined />
              <span>{autoplay ? "自动播放 开" : "自动播放 关"}</span>
            </div>
            {hasNext && (
              <div className={styles.footerBtn} onClick={handlePlayNext}>
                <StepForwardOutlined />
                <span>下一集</span>
              </div>
            )}
          </div>
        </div>
      </div>

      <div className={styles.infoArea}>
        <div className={styles.introHeading}>剧情简介</div>
        <div className={styles.intro}>
          {detail.descriptor.content ? detail.descriptor.content.replace(/<[^>]+>/g, "").trim() : "暂无简介"}
        </div>
      </div>

      <div className={styles.recommendation}>
        <h2 className={styles.sectionTitle}>相关推荐</h2>
        <FilmList list={relate} className={styles.classifyGrid} />
      </div>
    </div>
  );
}

export default function PlayPage() {
  return (
    <Suspense fallback={<AppLoading />}>
      <PlayerContent />
    </Suspense>
  );
}
