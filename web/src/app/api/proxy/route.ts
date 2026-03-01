import { NextRequest, NextResponse } from "next/server";

/**
 * 通用视频代理路由：
 * 采用基础透明转发，移除模拟浏览器头，仅保留必要的跨域注入逻辑。
 */
export async function GET(req: NextRequest) {
  const { searchParams } = new URL(req.url);
  const targetUrl = searchParams.get("url");

  if (!targetUrl) {
    return new NextResponse("Missing url parameter", { status: 400 });
  }

  try {
    const urlObj = new URL(targetUrl);

    // 伪装头部：针对 ffzy 等资源站的防盗链进行欺骗
    const proxyHeaders: Record<string, string> = {
      "User-Agent":
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
      Referer: `${urlObj.protocol}//${urlObj.host}/`,
      Origin: `${urlObj.protocol}//${urlObj.host}`,
    };

    const response = await fetch(targetUrl, {
      method: "GET",
      headers: proxyHeaders,
      cache: "no-store",
    });

    if (!response.ok) {
      console.error(`Proxy failed for ${targetUrl}: ${response.status}`);
      return new NextResponse(
        `Remote server responded with ${response.status}`,
        { status: response.status },
      );
    }

    return new NextResponse(response.body, {
      headers: {
        "Content-Type":
          response.headers.get("Content-Type") || "application/octet-stream",
        "Access-Control-Allow-Origin": "*",
        "Cache-Control": "public, max-age=3600",
      },
    });
  } catch (error: any) {
    console.error("Proxy error:", error.message);
    return new NextResponse(`Proxy error: ${error.message}`, { status: 500 });
  }
}
