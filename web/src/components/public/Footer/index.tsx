"use client";

import React from "react";
import { useRouter, usePathname } from "next/navigation";
import {
  HomeOutlined,
  SyncOutlined,
  HistoryOutlined,
  HeartOutlined,
  UserOutlined,
} from "@ant-design/icons";
import styles from "./index.module.less";

export default function Footer() {
  return (
    <footer className={styles.footer}>
      <p>
        本站所有内容均来自互联网分享站点所提供的公开引用资源，未提供资源上传、存储服务。
      </p>
    </footer>
  );
}
