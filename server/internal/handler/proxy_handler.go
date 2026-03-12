package handler

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type ProxyHandler struct {
	mu          sync.RWMutex
	proxyStr    string
	proxyClient *http.Client
}

var ProxyHd = &ProxyHandler{}

// 标准桌面浏览器 UA
const proxyUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"

func (h *ProxyHandler) getClient() *http.Client {
	h.mu.RLock()
	if h.proxyClient != nil {
		client := h.proxyClient
		h.mu.RUnlock()
		return client
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.proxyClient != nil {
		return h.proxyClient
	}

	h.proxyClient = &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     90 * time.Second,
		},
		Timeout: 60 * time.Second,
	}
	return h.proxyClient
}

func (h *ProxyHandler) ProxyVideo(c *gin.Context) {
	target := c.Query("url")
	if target == "" {
		c.AbortWithStatus(400)
		return
	}

	req, _ := http.NewRequestWithContext(c.Request.Context(), "GET", target, nil)
	req.Header.Set("User-Agent", proxyUA)

	resp, err := h.getClient().Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		c.AbortWithStatus(502)
		return
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	// 只有 M3U8 需要特殊解析重写
	isM3U8 := strings.Contains(target, ".m3u8") || strings.Contains(contentType, "mpegurl")

	if !isM3U8 {
		c.Header("Content-Type", contentType)
		_, _ = io.Copy(c.Writer, resp.Body)
		return
	}

	// M3U8 路径补全重写：大道至简
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	baseURL, _ := url.Parse(target)
	scanner := bufio.NewScanner(resp.Body)
	// 针对含有内联 KEY 的超长行扩大缓冲区
	scanner.Buffer(make([]byte, 512*1024), 512*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			fmt.Fprintln(c.Writer, "")
			continue
		}

		// 1. 匹配分片/播放列表行 (非 # 开头)
		if !strings.HasPrefix(line, "#") {
			line = h.wrap(baseURL, line)
		} else if strings.Contains(line, "URI=\"") {
			// 2. 匹配标签内的 URI (如 #EXT-X-KEY:URI="...")
			line = h.rewriteTagURI(baseURL, line)
		}
		fmt.Fprintln(c.Writer, line)
	}
}

// wrap 将原始 URL 转换为代理 URL，自动处理相对/绝对路径
func (h *ProxyHandler) wrap(base *url.URL, ref string) string {
	if ref == "" || strings.HasPrefix(ref, "data:") {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return "/api/proxy/video?url=" + url.QueryEscape(base.ResolveReference(u).String())
}

// rewriteTagURI 查找并替换字符串中的 URI="..." 部分
func (h *ProxyHandler) rewriteTagURI(base *url.URL, line string) string {
	start := strings.Index(line, "URI=\"") + 5
	end := strings.Index(line[start:], "\"") + start
	if start < 5 || end <= start {
		return line
	}
	uri := line[start:end]
	return line[:start] + h.wrap(base, uri) + line[end:]
}
