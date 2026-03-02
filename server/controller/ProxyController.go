package controller

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// ProxyVideo 极简视频代理
func ProxyVideo(c *gin.Context) {
	targetUrl := c.Query("url")
	if targetUrl == "" {
		c.AbortWithStatus(400)
		return
	}

	// 1. 创建请求并转发用户 Agent，只补一个 Referer 绕过防盗链
	req, _ := http.NewRequest("GET", targetUrl, nil)
	req.Header.Set("User-Agent", c.Request.UserAgent())
	if u, err := url.Parse(targetUrl); err == nil {
		req.Header.Set("Referer", u.Scheme+"://"+u.Host+"/")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.AbortWithStatus(502)
		return
	}
	defer resp.Body.Close()

	// 2. 基础响应头转发
	c.Header("Access-Control-Allow-Origin", "*")
	contentType := resp.Header.Get("Content-Type")
	c.Header("Content-Type", contentType)

	// 3. 判断是否为 M3U8，只有 M3U8 需要特殊处理路径补全
	isM3U8 := strings.Contains(targetUrl, ".m3u8") || strings.Contains(contentType, "mpegurl")

	if isM3U8 {
		// 获取基础目录用于补全相对路径
		u, _ := url.Parse(targetUrl)
		baseDir := targetUrl[:strings.LastIndex(targetUrl, "/")+1]
		baseUrl := u.Scheme + "://" + u.Host

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			// 只处理非注释且非空的行（通常是 TS 路径）
			if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
				fullPath := line
				if !strings.HasPrefix(line, "http") {
					if strings.HasPrefix(line, "/") {
						fullPath = baseUrl + line
					} else {
						fullPath = baseDir + line
					}
				}
				// 补全后再次通过代理访问
				line = "/proxy/video?url=" + url.QueryEscape(fullPath)
			}
			fmt.Fprintln(c.Writer, line)
		}
	} else {
		// 非 M3U8（如 TS 分片）直接 io.Copy，不处理内容
		if resp.Header.Get("Content-Length") != "" {
			c.Header("Content-Length", resp.Header.Get("Content-Length"))
		}
		io.Copy(c.Writer, resp.Body)
	}
}
