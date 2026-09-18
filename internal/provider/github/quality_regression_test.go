package github

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	gh "github.com/google/go-github/v57/github"
)

type fixedTransport func(*http.Request) (*http.Response, error)

func (f fixedTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func Test_文件分页_最后一页结束且完整返回(t *testing.T) {
	remaining := 2
	transport := fixedTransport(func(r *http.Request) (*http.Response, error) {
		// 限制桩的响应数量，使错误的重复分页立即失败，不依赖真实网络或超时。
		if remaining == 0 {
			return nil, errors.New("末页之后仍然请求")
		}
		remaining--
		header := make(http.Header)
		body := `[{"filename":"second.go"}]`
		if r.URL.Query().Get("page") == "" {
			header.Set("Link", `<https://api.github.com/repos/o/r/pulls/1/files?page=2>; rel="next"`)
			body = `[{"filename":"first.go"}]`
		}
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	client := NewWithClient(gh.NewClient(&http.Client{Transport: transport}))

	files, err := client.ListPRFiles(context.Background(), "o", "r", 1)

	if err != nil || len(files) != 2 || files[0].GetFilename() != "first.go" || files[1].GetFilename() != "second.go" {
		t.Fatalf("文件 = %+v，错误 = %v", files, err)
	}
}

func Test_账号列表_解析无域名前缀的仓库地址(t *testing.T) {
	transport := fixedTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"items":[{"number":7,"state":"open","title":"测试","pull_request":{},"repository_url":"/repos/owner/project"}]}`
		if r.URL.Path == "/user" {
			body = `{"login":"tester"}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	client := NewWithClient(gh.NewClient(&http.Client{Transport: transport}))

	items, err := client.ListMyPullRequests(context.Background())

	if err != nil || len(items) != 1 || items[0].RepoFull != "owner/project" {
		t.Fatalf("条目 = %+v，错误 = %v", items, err)
	}
}
