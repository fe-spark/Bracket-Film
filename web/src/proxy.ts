import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";

export default function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;

  // 仅拦截 /manage 路径
  if (pathname.startsWith("/manage")) {
    // 从 cookie 中读取 auth token
    const authCookie = request.cookies.get("auth-token");
    if (!authCookie?.value) {
      return NextResponse.redirect(new URL("/login", request.url));
    }
  }

  return NextResponse.next();
}

export const config = {
  matcher: ["/manage/:path*"],
};
