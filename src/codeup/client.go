package codeup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

const codeupBaseURL = "https://openapi-rdc.aliyuncs.com"

type CodeupClient struct {
	baseURL    string
	orgID      string
	token      string
	http       *http.Client
	requestCnt atomic.Int64
}

type Repository struct {
	ID                int64  `json:"id"`
	Name              string `json:"name"`
	Path              string `json:"path"`
	NameWithNamespace string `json:"nameWithNamespace"`
	PathWithNamespace string `json:"pathWithNamespace"`
}

type Branch struct {
	Name      string `json:"name"`
	Protected any    `json:"protected"`
	Commit    *struct {
		ID      string `json:"id"`
		ShortID string `json:"short_id"`
	} `json:"commit"`
}

type CompareDetailResponse struct {
	Commits []json.RawMessage `json:"commits"`
	Message string            `json:"message"`
	Error   string            `json:"error"`
}

type apiErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func NewCodeupClient(orgID, token string) *CodeupClient {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	}
	return &CodeupClient{
		baseURL: codeupBaseURL,
		orgID:   orgID,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second, Transport: transport},
	}
}

func (c *CodeupClient) ListRepositories(ctx context.Context, search string, page, perPage int64) ([]Repository, error) {
	query := url.Values{}
	query.Set("page", strconv.FormatInt(page, 10))
	query.Set("perPage", strconv.FormatInt(perPage, 10))
	query.Set("search", search)
	query.Set("archived", "false")

	var repos []Repository
	if err := c.doJSON(ctx, http.MethodGet, c.orgPath("repositories"), query, &repos); err != nil {
		return nil, err
	}
	return repos, nil
}

func (c *CodeupClient) ListBranches(ctx context.Context, repositoryIdentity string, page, perPage int64) ([]Branch, error) {
	query := url.Values{}
	query.Set("page", strconv.FormatInt(page, 10))
	query.Set("perPage", strconv.FormatInt(perPage, 10))

	var branches []Branch
	path := c.repoPath(repositoryIdentity, "branches")
	if err := c.doJSON(ctx, http.MethodGet, path, query, &branches); err != nil {
		return nil, err
	}
	return branches, nil
}

func (c *CodeupClient) GetBranch(ctx context.Context, repositoryIdentity, branchName string) (*Branch, error) {
	path := c.repoPath(repositoryIdentity, "branches", branchName)
	var branch Branch
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &branch); err != nil {
		return nil, err
	}
	return &branch, nil
}

func (c *CodeupClient) GetCompareDetail(ctx context.Context, repositoryIdentity, from, to string) (*CompareDetailResponse, error) {
	query := url.Values{}
	query.Set("from", from)
	query.Set("to", to)
	query.Set("sourceType", "branch")
	query.Set("targetType", "branch")
	query.Set("straight", "false")

	var result CompareDetailResponse
	if err := c.doJSON(ctx, http.MethodGet, c.repoPath(repositoryIdentity, "compares"), query, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *CodeupClient) ListChangeRequests(ctx context.Context) ([]ChangeRequest, error) {
	var result []ChangeRequest
	path := c.orgPath("changeRequests")
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, fmt.Errorf("list change requests: %w", err)
	}
	return result, nil
}

func (c *CodeupClient) DeleteBranch(ctx context.Context, repositoryIdentity, branchName string) error {
	path := c.repoPath(repositoryIdentity, "branches", branchName)
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil)
}

func (c *CodeupClient) RequestCount() int64 {
	return c.requestCnt.Load()
}

func (c *CodeupClient) orgPath(parts ...string) string {
	pathParts := []string{"oapi", "v1", "codeup", "organizations", c.orgID}
	pathParts = append(pathParts, parts...)
	return joinEscapedPath(pathParts...)
}

func (c *CodeupClient) repoPath(repositoryIdentity string, parts ...string) string {
	pathParts := []string{"repositories", repositoryIdentity}
	pathParts = append(pathParts, parts...)
	return c.orgPath(pathParts...)
}

func (c *CodeupClient) doJSON(ctx context.Context, method, path string, query url.Values, out any) error {
	c.requestCnt.Add(1)

	reqURL, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	reqURL.Path = path
	if query != nil {
		reqURL.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-yunxiao-token", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var apiErr apiErrorResponse
		if decodeErr := json.NewDecoder(resp.Body).Decode(&apiErr); decodeErr != nil {
			return fmt.Errorf("HTTP %d (错误响应解析失败: %v)", resp.StatusCode, decodeErr)
		}
		if apiErr.Error != "" || apiErr.Message != "" {
			return fmt.Errorf("HTTP %d: %s%s", resp.StatusCode, apiErr.Error, apiErr.Message)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func joinEscapedPath(parts ...string) string {
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(part))
	}
	return "/" + strings.Join(escaped, "/")
}

func (c *CodeupClient) doJSONWithBody(ctx context.Context, method, path string, body any, out any) error {
	c.requestCnt.Add(1)

	reqURL, err := url.Parse(c.baseURL)
	if err != nil {
		return err
	}
	reqURL.Path = path

	var bodyReader *strings.Reader
	if body != nil {
		bodyBytes, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = strings.NewReader(string(bodyBytes))
	} else {
		bodyReader = strings.NewReader("")
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-yunxiao-token", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	if out == nil {
		return nil
	}
	return json.Unmarshal(respBody, out)
}

func (c *CodeupClient) CreateChangeRequest(ctx context.Context, repoIdentity string, req CreateChangeRequestReq) (*ChangeRequest, error) {
	path := c.repoPath(repoIdentity, "changeRequests")
	var result ChangeRequest
	if err := c.doJSONWithBody(ctx, http.MethodPost, path, req, &result); err != nil {
		return nil, fmt.Errorf("create change request: %w", err)
	}
	return &result, nil
}

func (c *CodeupClient) ReviewChangeRequest(ctx context.Context, repoIdentity string, localID int, req ReviewChangeRequestReq) error {
	path := c.repoPath(repoIdentity, "changeRequests", fmt.Sprintf("%d", localID), "review")
	return c.doJSONWithBody(ctx, http.MethodPost, path, req, nil)
}

func (c *CodeupClient) MergeChangeRequest(ctx context.Context, repoIdentity string, localID int, req MergeChangeRequestReq) (*ChangeRequest, error) {
	path := c.repoPath(repoIdentity, "changeRequests", fmt.Sprintf("%d", localID), "merge")
	var result ChangeRequest
	if err := c.doJSONWithBody(ctx, http.MethodPost, path, req, &result); err != nil {
		return nil, fmt.Errorf("merge change request: %w", err)
	}
	return &result, nil
}
