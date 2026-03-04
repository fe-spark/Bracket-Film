package util

import (
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

/*
网络请求, 数据爬取
*/

var (
	Client = CreateClient()
)

// RequestInfo 请求参数结构体
type RequestInfo struct {
	Uri    string      `json:"uri"`    // 请求url地址
	Params url.Values  `json:"param"`  // 请求参数
	Header http.Header `json:"header"` // 请求头数据
	Resp   []byte      `json:"resp"`   // 响应结果数据
	Err    string      `json:"err"`    // 错误信息
}

// RefererUrl 记录上次请求的url
var RefererUrl string

// CreateClient 初始化请求客户端
func CreateClient() *colly.Collector {
	c := colly.NewCollector()

	// 设置请求使用clash的socks5代理
	//setProxy(c)

	// 设置代理信息
	//if proxy, err := proxy.RoundRobinProxySwitcher("127.0.0.1:7890"); err != nil {
	//	c.SetProxyFunc(proxy)
	//}
	// 设置并发数量控制
	//c.Async = true
	// 访问深度
	c.MaxDepth = 1
	//可重复访问
	c.AllowURLRevisit = true
	// 设置超时时间 默认10s
	c.SetRequestTimeout(20 * time.Second)
	// 发起请求之前会调用的方法
	c.OnRequest(func(request *colly.Request) {
		// 设置一些请求头信息
		// request.Headers.Set("Content-Type", "application/json;charset=UTF-8") // GET 请求通常不需要此头，且可能导致部分 API 报 Bad Request
		request.Headers.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36")
		//request.Headers.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7")
		// 请求完成后设置请求头Referer
		if len(RefererUrl) > 0 && strings.Contains(RefererUrl, request.URL.Host) {
			request.Headers.Set("Referer", RefererUrl)
		}
	})
	// 请求期间报错的回调
	c.OnError(func(response *colly.Response, err error) {
		log.Printf("请求异常: URL: %s Error: %s\n", response.Request.URL, err)
	})
	return c
}

// ApiGet 请求数据的方法
func ApiGet(r *RequestInfo) {
	// 每次请求使用独立的事件处理，防止全局回调累积
	c := Client.Clone()

	if r.Header != nil {
		if t, err := strconv.Atoi(r.Header.Get("timeout")); err != nil && t > 0 {
			c.SetRequestTimeout(time.Duration(t) * time.Second)
		}
	}
	// 设置随机请求头
	extensions.RandomUserAgent(c)

	// 请求成功后的响应
	c.OnResponse(func(response *colly.Response) {
		if (response.StatusCode == 200 || (response.StatusCode >= 300 && response.StatusCode <= 399)) && len(response.Body) > 0 {
			r.Resp = response.Body
		} else {
			r.Resp = []byte{}
		}
		RefererUrl = response.Request.URL.String()
	})

	// 构造完整 URL
	targetUrl := buildUrl(r.Uri, r.Params)

	// 执行请求
	err := c.Visit(targetUrl)
	if err != nil {
		r.Err = err.Error()
		log.Println("获取数据失败: ", err)
	}
}

// ApiTest 处理API请求后的数据, 主测试
func ApiTest(r *RequestInfo) error {
	// 测试时使用完全独立的 Collector，避免状态污染
	c := CreateClient()

	// 请求成功后的响应
	c.OnResponse(func(response *colly.Response) {
		if (response.StatusCode == 200 || (response.StatusCode >= 300 && response.StatusCode <= 399)) && len(response.Body) > 0 {
			r.Resp = response.Body
		} else {
			r.Resp = []byte{}
		}
	})

	targetUrl := buildUrl(r.Uri, r.Params)
	err := c.Visit(targetUrl)
	if err != nil {
		log.Printf("ApiTest 访问失败: %s, Error: %v\n", targetUrl, err)
	}
	return err
}

// buildUrl 安全地拼接 URL 和参数
func buildUrl(base string, params url.Values) string {
	if len(params) == 0 {
		return base
	}
	u, err := url.Parse(base)
	if err != nil {
		// 回退方案
		if strings.Contains(base, "?") {
			return base + "&" + params.Encode()
		}
		return base + "?" + params.Encode()
	}
	q := u.Query()
	for k, v := range params {
		for _, val := range v {
			q.Set(k, val)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// 本地代理测试
func setProxy(c *colly.Collector) {
	proxyUrl, _ := url.Parse("socks5://127.0.0.1:7890")
	c.WithTransport(&http.Transport{Proxy: http.ProxyURL(proxyUrl)})
}
