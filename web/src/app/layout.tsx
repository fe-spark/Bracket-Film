import type { Metadata } from "next";
import { AntdRegistry } from "@ant-design/nextjs-registry";
import { Outfit } from "next/font/google";
import GlobalThemeProvider from "@/components/theme/GlobalThemeProvider";
import "./globals.css";
const outfit = Outfit({
  subsets: ["latin"],
  display: "swap",
  weight: ["300", "400", "500", "600", "700", "800", "900"],
});

export async function generateMetadata(): Promise<Metadata> {
  let siteName = "";
  let describe = "";
  let keyword = "";
  let icon = "";

  try {
    const apiUrl = process.env.API_URL || "http://127.0.0.1:3601";
    const res = await fetch(`${apiUrl}/config/basic`, {
      next: { revalidate: 60 },
    });
    if (res.ok) {
      const json = await res.json();
      if (json.code === 0 && json.data) {
        siteName = json.data.siteName || "";
        describe = json.data.describe || "";
        keyword = json.data.keyword || "";
        icon = json.data.logo || "";
      }
    }
  } catch (err) {
    console.error("fetch metadata error:", err);
  }

  const generated: Metadata = {};
  if (siteName) generated.title = siteName;
  if (describe) generated.description = describe;
  if (keyword) generated.keywords = keyword;
  if (icon) generated.icons = { icon };

  return generated;
}

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="zh-CN">
      <body className={outfit.className}>
        <AntdRegistry>
          <GlobalThemeProvider fontFamily={outfit.style.fontFamily}>
            {children}
          </GlobalThemeProvider>
        </AntdRegistry>
      </body>
    </html>
  );
}
