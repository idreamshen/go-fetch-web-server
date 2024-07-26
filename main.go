package main

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/k3a/html2text"
)

type FetchReq struct {
	Urls []string `json:"urls"`
}

type Resp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Contents []string `json:"contents"`
	} `json:"data"`
}

func main() {

	r := gin.New()
	r.Use(gzip.Gzip(gzip.DefaultCompression))

	r.POST("/fetch", func(c *gin.Context) {
		req := FetchReq{}
		ctx := c.Request.Context()

		if err := c.ShouldBindJSON(&req); err != nil {
			c.Error(err)
			c.AbortWithStatusJSON(http.StatusBadRequest, Resp{Code: 400, Msg: err.Error()})
			return
		}

		urls := req.Urls
		contents := []string{}
		wg := sync.WaitGroup{}
		results := make(chan string, len(urls))
		for _, v := range urls {
			wg.Add(1)
			go crawlURL(ctx, v, &wg, results)
		}

		wg.Wait()
		close(results)
		for v := range results {
			contents = append(contents, v)
		}

		resp := Resp{}
		resp.Data.Contents = contents

		c.JSON(http.StatusOK, resp)
	})

	r.Run()

}

func crawlURL(ctx context.Context, url string, wg *sync.WaitGroup, results chan<- string) {
	defer wg.Done()

	proxyUrl := os.Getenv("CRAWLER_PROXY")
	var body = fetchWithProxy(url, proxyUrl)

	if len(body) == 0 {
		return
	}

	txt := html2text.HTML2Text(body)
	if len(txt) == 0 {
		return
	}

	results <- txt
}

func fetchWithProxy(webUrl, proxyUrl string) string {
	/*
		1. 代理请求
		2. 跳过https不安全验证
		3. 自定义请求头 User-Agent
	*/
	// webUrl := "http://ip.gs/"
	// proxyUrl := "http://171.215.227.125:9000"

	request, _ := http.NewRequest("GET", webUrl, nil)
	request.Header.Set("Connection", "keep-alive")
	request.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/56.0.2924.87 Safari/537.36")

	proxy, _ := url.Parse(proxyUrl)
	tr := &http.Transport{
		Proxy:           http.ProxyURL(proxy),
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   time.Second * 5, //超时时间
	}

	resp, err := client.Do(request)
	if err != nil {
		return ""
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return string(body)
}
