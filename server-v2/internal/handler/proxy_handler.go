package handler

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

type ProxyHandler struct{}

var ProxyHd = new(ProxyHandler)

func (h *ProxyHandler) ProxyVideo(c *gin.Context) {
	targetUrl := c.Query("url")
	if targetUrl == "" {
		c.AbortWithStatus(400)
		return
	}

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

	c.Header("Access-Control-Allow-Origin", "*")
	contentType := resp.Header.Get("Content-Type")
	c.Header("Content-Type", contentType)

	isM3U8 := strings.Contains(targetUrl, ".m3u8") || strings.Contains(contentType, "mpegurl")

	if isM3U8 {
		u, _ := url.Parse(targetUrl)
		baseDir := targetUrl[:strings.LastIndex(targetUrl, "/")+1]
		baseUrl := u.Scheme + "://" + u.Host

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			originalLine := line

			if !strings.HasPrefix(line, "#") && strings.TrimSpace(line) != "" {
				fullPath := resolveURL(line, baseDir, baseUrl)
				line = "/api/proxy/video?url=" + url.QueryEscape(fullPath)
			} else if strings.HasPrefix(line, "#") && strings.Contains(line, "URI=\"") {
				start := strings.Index(line, "URI=\"") + 5
				end := strings.Index(line[start:], "\"") + start
				if end > start {
					uri := line[start:end]
					fullPath := resolveURL(uri, baseDir, baseUrl)
					proxiedURI := "/api/proxy/video?url=" + url.QueryEscape(fullPath)
					line = line[:start] + proxiedURI + line[end:]
				}
			}

			if line != "" || originalLine == "" {
				fmt.Fprintln(c.Writer, line)
			}
		}
	} else {
		if resp.Header.Get("Content-Length") != "" {
			c.Header("Content-Length", resp.Header.Get("Content-Length"))
		}
		_, _ = io.Copy(c.Writer, resp.Body)
	}
}

func resolveURL(path, baseDir, baseUrl string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	if strings.HasPrefix(path, "/") {
		return baseUrl + path
	}
	return baseDir + path
}
