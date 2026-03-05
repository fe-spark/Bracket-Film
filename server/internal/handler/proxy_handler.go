package handler

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ProxyHandler struct{}

var ProxyHd = new(ProxyHandler)

// 标准桌面浏览器 UA：ffzy / bfikuncdn / rsfcxq / yuglf / 360zyx 等 CDN 均对移动端 UA 较敏感
const proxyUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

// proxyTransport 共享连接池，复用 TCP 连接降低握手开销
var proxyTransport = &http.Transport{
	MaxIdleConns:          100,
	MaxIdleConnsPerHost:   10,
	IdleConnTimeout:       90 * time.Second,
	ResponseHeaderTimeout: 10 * time.Second,
}

// m3u8Client：索引文件小，5s 足够
var m3u8Client = &http.Client{
	Timeout:   10 * time.Second,
	Transport: proxyTransport,
}

// tsClient：TS 分片 3-8 MB，不设 Timeout（用 context 控制），只限响应头
var tsClient = &http.Client{
	Transport: proxyTransport,
}

func (h *ProxyHandler) ProxyVideo(c *gin.Context) {
	targetUrl := c.Query("url")
	if targetUrl == "" {
		c.AbortWithStatus(400)
		return
	}
	// 兼容历史脏数据：ConvertPlayUrl 修复前已入库的 "$http://..." 链接
	// 新采集数据已在 ConvertPlayUrl 中统一清洗，此处仅作保底
	targetUrl = strings.TrimLeft(targetUrl, "$")

	// 判断请求类型：m3u8 索引 / key 文件 / TS 分片
	isM3U8Request := strings.Contains(targetUrl, ".m3u8")
	isTSRequest := strings.Contains(targetUrl, ".ts")

	req, err := http.NewRequest("GET", targetUrl, nil)
	if err != nil {
		c.AbortWithStatus(400)
		return
	}
	req.Header.Set("User-Agent", proxyUA)
	req.Header.Set("Accept", "*/*")

	// TS 分片使用带 context 超时的 tsClient，避免 Timeout 截断流式传输
	var resp *http.Response
	if isTSRequest {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
		defer cancel()
		resp, err = tsClient.Do(req.WithContext(ctx))
	} else {
		resp, err = m3u8Client.Do(req)
	}
	if err != nil {
		c.AbortWithStatus(502)
		return
	}
	defer resp.Body.Close()

	// 转发非 200 状态码，让客户端可感知 403/404/429
	if resp.StatusCode != http.StatusOK {
		c.Status(resp.StatusCode)
		return
	}

	contentType := resp.Header.Get("Content-Type")

	// 上游返回 HTML 说明是错误页（403 页/share 网页/防盗链跳转），不能流给播放器
	if strings.Contains(contentType, "text/html") {
		c.AbortWithStatus(502)
		return
	}

	c.Header("Access-Control-Allow-Origin", "*")

	// 判断是否为 M3U8 内容：看 URL 后缀 + Content-Type
	isM3U8Content := isM3U8Request ||
		strings.Contains(contentType, "mpegurl") ||
		strings.Contains(contentType, "x-mpegurl")

	if isM3U8Content {
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		u, _ := url.Parse(targetUrl)
		baseDir := targetUrl[:strings.LastIndex(targetUrl, "/")+1]
		baseUrl := u.Scheme + "://" + u.Host

		// 扩大 Scanner 缓冲到 512KB，防止含 base64 key 的 URI 行超过默认 64KB 被截断
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 512*1024), 512*1024)

		for scanner.Scan() {
			line := scanner.Text()
			originalLine := line

			if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
				// 分片路径（绝对或相对）统一补全后代理
				fullPath := resolveURL(line, baseDir, baseUrl)
				line = "/api/proxy/video?url=" + url.QueryEscape(fullPath)
			} else if strings.HasPrefix(line, "#") && strings.Contains(line, "URI=\"") {
				// 标签内 URI（#EXT-X-KEY key.key / #EXT-X-MAP init.mp4 等）
				start := strings.Index(line, "URI=\"") + 5
				end := strings.Index(line[start:], "\"") + start
				if end > start {
					uri := line[start:end]
					// data: URI 是内联数据，不需要代理
					if !strings.HasPrefix(uri, "data:") {
						fullPath := resolveURL(uri, baseDir, baseUrl)
						proxiedURI := "/api/proxy/video?url=" + url.QueryEscape(fullPath)
						line = line[:start] + proxiedURI + line[end:]
					}
				}
			}

			if line != "" || originalLine == "" {
				fmt.Fprintln(c.Writer, line)
			}
		}
	} else {
		// TS 分片 / key 文件 / mp4：流式透传，不设 Content-Length（避免 chunked 截断）
		c.Header("Content-Type", contentType)
		_, _ = io.Copy(c.Writer, resp.Body)
	}
}

// resolveURL 将相对路径补全为绝对路径
// 支持：绝对 URL / 根路径（/path）/ ../相对路径 / 普通相对路径
func resolveURL(rawPath, baseDir, baseUrl string) string {
	if strings.HasPrefix(rawPath, "http://") || strings.HasPrefix(rawPath, "https://") {
		return rawPath
	}
	// 分离查询参数，避免 "seg.ts?token=x" 被路径处理错误
	pathOnly, query := rawPath, ""
	if idx := strings.Index(rawPath, "?"); idx != -1 {
		pathOnly, query = rawPath[:idx], rawPath[idx:]
	}
	var resolved string
	if strings.HasPrefix(pathOnly, "/") {
		resolved = baseUrl + pathOnly
	} else {
		// path.Clean 处理 ../ 跳级路径
		resolved = path.Clean(baseDir + pathOnly)
		// path.Clean 会把 https:// 压缩成 https:/，需还原
		if strings.HasPrefix(resolved, "https:/") && !strings.HasPrefix(resolved, "https://") {
			resolved = "https://" + resolved[7:]
		} else if strings.HasPrefix(resolved, "http:/") && !strings.HasPrefix(resolved, "http://") {
			resolved = "http://" + resolved[6:]
		}
	}
	return resolved + query
}
