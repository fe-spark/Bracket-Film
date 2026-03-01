"use client";

import React, { Suspense } from "react";
import Header from "@/components/public/Header";
import Footer from "@/components/public/Footer";
import styles from "./layout.module.less";

export default function PublicLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div className={styles.layoutWrapper}>
      <Suspense fallback={null}>
        <Header />
      </Suspense>
      <main className={`${styles.publicMain} page-entry`}>{children}</main>
      <Footer />
    </div>
  );
}
