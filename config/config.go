package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

const (
	defaultConfigName = "config"
	defaultConfigType = "yaml"
)

type Config struct {
	Archery         ArcheryConfig `mapstructure:"archery"`
	PollInterval    int           `mapstructure:"poll_interval"`
	LogLevel        string        `mapstructure:"log_level"`
	MaxConcurrent   int           `mapstructure:"max_concurrent"`
	ApprovalRemark  string        `mapstructure:"approval_remark"`
	Approver        string        `mapstructure:"approver"`
	PendingStatuses []string      `mapstructure:"pending_statuses"`
	RetryCount      int           `mapstructure:"retry_count"`
	RetryBackoffSec int           `mapstructure:"retry_backoff_sec"`
	Health          HealthConfig  `mapstructure:"health"`
}

type ArcheryConfig struct {
	BaseURL             string `mapstructure:"base_url"`
	Username            string `mapstructure:"username"`
	Password            string `mapstructure:"password"`
	Token               string `mapstructure:"token"`
	RefreshToken        string `mapstructure:"refresh_token"`
	TokenTTL            int    `mapstructure:"token_ttl"`
	AuthScheme          string `mapstructure:"auth_scheme"`
	WorkflowListPath    string `mapstructure:"workflow_list_path"`
	WorkflowApprovePath string `mapstructure:"workflow_approve_path"`
	WorkflowApproveAlt  string `mapstructure:"workflow_approve_alt"`
	TokenPath           string `mapstructure:"token_path"`
	TokenRefreshPath    string `mapstructure:"token_refresh_path"`
	LoginPath           string `mapstructure:"login_path"`
}

type HealthConfig struct {
	Enabled bool `mapstructure:"enabled"`
	Port    int  `mapstructure:"port"`
}

func Load() (*Config, error) {
	_ = loadDotEnv(".env")

	v := viper.New()
	setDefaults(v)
	v.SetConfigName(defaultConfigName)
	v.SetConfigType(defaultConfigType)
	v.AddConfigPath(".")
	v.SetEnvPrefix("ARCHERY_AUTO_APPROVE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("poll_interval", 300)
	v.SetDefault("log_level", "info")
	v.SetDefault("max_concurrent", 5)
	v.SetDefault("approval_remark", "系统自动审批（非工作时间）")
	v.SetDefault("approver", "")
	v.SetDefault("pending_statuses", []string{"workflow_manreviewing"})
	v.SetDefault("retry_count", 3)
	v.SetDefault("retry_backoff_sec", 2)
	v.SetDefault("archery.base_url", "")
	v.SetDefault("archery.username", "")
	v.SetDefault("archery.password", "")
	v.SetDefault("archery.token", "")
	v.SetDefault("archery.refresh_token", "")
	v.SetDefault("archery.token_ttl", int(time.Hour.Seconds()))
	v.SetDefault("archery.auth_scheme", "Bearer")
	v.SetDefault("archery.workflow_list_path", "/api/v1/workflow/")
	v.SetDefault("archery.workflow_approve_path", "/api/v1/workflow/audit/")
	v.SetDefault("archery.workflow_approve_alt", "/api/workflow/approve/")
	v.SetDefault("archery.token_path", "/api/auth/token/")
	v.SetDefault("archery.token_refresh_path", "/api/auth/token/refresh/")
	v.SetDefault("archery.login_path", "/api/v1/user/login/")
	v.SetDefault("health.enabled", true)
	v.SetDefault("health.port", 8080)
}

func (c *Config) Validate() error {
	if strings.TrimSpace(c.Archery.BaseURL) == "" {
		return fmt.Errorf("archery.base_url is required")
	}
	if strings.TrimSpace(c.Archery.Token) == "" {
		if strings.TrimSpace(c.Archery.Username) == "" || strings.TrimSpace(c.Archery.Password) == "" {
			return fmt.Errorf("archery.token or archery.username/password is required, you can place them in .env")
		}
	}
	if strings.TrimSpace(c.Archery.AuthScheme) == "" {
		return fmt.Errorf("archery.auth_scheme must not be empty")
	}
	if c.PollInterval <= 0 {
		return fmt.Errorf("poll_interval must be > 0")
	}
	if c.MaxConcurrent <= 0 {
		return fmt.Errorf("max_concurrent must be > 0")
	}
	if c.RetryCount <= 0 {
		return fmt.Errorf("retry_count must be > 0")
	}
	if c.RetryBackoffSec < 0 {
		return fmt.Errorf("retry_backoff_sec must be >= 0")
	}
	if len(c.PendingStatuses) == 0 {
		return fmt.Errorf("pending_statuses must not be empty")
	}
	return nil
}

func (c *Config) PollDuration() time.Duration {
	return time.Duration(c.PollInterval) * time.Second
}

func (c *Config) RetryBackoff() time.Duration {
	return time.Duration(c.RetryBackoffSec) * time.Second
}

func loadDotEnv(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return godotenv.Overload(path)
}
