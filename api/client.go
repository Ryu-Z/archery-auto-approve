package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"archery-auto-approve/config"
	"archery-auto-approve/utils"
)

type Client struct {
	cfg        *config.Config
	httpClient *http.Client
	logger     utils.Logger

	tokenMu      sync.RWMutex
	token        string
	refreshToken string
	tokenUntil   time.Time
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token        string `json:"token"`
	AccessToken  string `json:"access"`
	RefreshToken string `json:"refresh"`
}

type refreshRequest struct {
	Refresh string `json:"refresh"`
}

func NewClient(cfg *config.Config, logger utils.Logger) *Client {
	client := &Client{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 20 * time.Second,
		},
		logger: logger,
	}
	if strings.TrimSpace(cfg.Archery.Token) != "" {
		client.token = strings.TrimSpace(cfg.Archery.Token)
		client.refreshToken = strings.TrimSpace(cfg.Archery.RefreshToken)
		client.tokenUntil = time.Now().Add(3650 * 24 * time.Hour)
	}
	return client
}

func (c *Client) ensureToken(ctx context.Context) error {
	c.tokenMu.RLock()
	token := c.token
	expireAt := c.tokenUntil
	c.tokenMu.RUnlock()

	if token != "" && time.Now().Before(expireAt) {
		return nil
	}

	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.token != "" && time.Now().Before(c.tokenUntil) {
		return nil
	}

	return c.loginLocked(ctx)
}

func (c *Client) loginLocked(ctx context.Context) error {
	if strings.TrimSpace(c.refreshToken) != "" {
		if err := c.refreshLocked(ctx); err == nil {
			return nil
		}
		c.logger.Warn("refresh token failed, fallback to password login")
	}

	reqBody := loginRequest{
		Username: c.cfg.Archery.Username,
		Password: c.cfg.Archery.Password,
	}

	respBody, _, err := c.doJSON(ctx, http.MethodPost, c.cfg.Archery.TokenPath, reqBody, false)
	if err != nil {
		c.logger.Warn("token endpoint login failed, fallback to session login", utils.FieldError(err))
		respBody, _, err = c.doJSON(ctx, http.MethodPost, c.cfg.Archery.LoginPath, reqBody, false)
		if err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
	}

	var loginResp loginResponse
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}
	token := strings.TrimSpace(loginResp.AccessToken)
	if token == "" {
		token = strings.TrimSpace(loginResp.Token)
	}
	if token == "" {
		return fmt.Errorf("empty token in login response")
	}

	c.token = token
	c.refreshToken = strings.TrimSpace(loginResp.RefreshToken)
	c.tokenUntil = time.Now().Add(time.Duration(c.cfg.Archery.TokenTTL) * time.Second)
	return nil
}

func (c *Client) refreshLocked(ctx context.Context) error {
	respBody, _, err := c.doJSON(ctx, http.MethodPost, c.cfg.Archery.TokenRefreshPath, refreshRequest{
		Refresh: c.refreshToken,
	}, false)
	if err != nil {
		return err
	}

	var refreshResp loginResponse
	if err := json.Unmarshal(respBody, &refreshResp); err != nil {
		return fmt.Errorf("decode refresh response: %w", err)
	}

	token := strings.TrimSpace(refreshResp.AccessToken)
	if token == "" {
		return fmt.Errorf("empty access token in refresh response")
	}

	c.token = token
	if strings.TrimSpace(refreshResp.RefreshToken) != "" {
		c.refreshToken = strings.TrimSpace(refreshResp.RefreshToken)
	}
	c.tokenUntil = time.Now().Add(time.Duration(c.cfg.Archery.TokenTTL) * time.Second)
	return nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, payload any, withAuth bool) ([]byte, int, error) {
	fullURL, err := joinURL(c.cfg.Archery.BaseURL, path)
	if err != nil {
		return nil, 0, err
	}

	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")

	if withAuth {
		if err := c.ensureToken(ctx); err != nil {
			return nil, 0, err
		}
		c.tokenMu.RLock()
		req.Header.Set("Authorization", c.cfg.Archery.AuthScheme+" "+c.token)
		c.tokenMu.RUnlock()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	if withAuth && resp.StatusCode == http.StatusUnauthorized {
		c.invalidateToken()
		return nil, resp.StatusCode, fmt.Errorf("unauthorized")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, resp.StatusCode, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	return data, resp.StatusCode, nil
}

func (c *Client) invalidateToken() {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if strings.TrimSpace(c.cfg.Archery.Token) != "" {
		return
	}
	c.token = ""
	c.refreshToken = ""
	c.tokenUntil = time.Time{}
}

func joinURL(base, path string) (string, error) {
	baseURL, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}
	ref, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parse path: %w", err)
	}
	return baseURL.ResolveReference(ref).String(), nil
}
