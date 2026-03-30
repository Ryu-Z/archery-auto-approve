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
	Archery         ArcheryConfig  `mapstructure:"archery"`
	Schedule        ScheduleConfig `mapstructure:"schedule"`
	PollInterval    int            `mapstructure:"poll_interval"`
	LogLevel        string         `mapstructure:"log_level"`
	MaxConcurrent   int            `mapstructure:"max_concurrent"`
	ApprovalRemark  string         `mapstructure:"approval_remark"`
	Approver        string         `mapstructure:"approver"`
	PendingStatuses []string       `mapstructure:"pending_statuses"`
	RetryCount      int            `mapstructure:"retry_count"`
	RetryBackoffSec int            `mapstructure:"retry_backoff_sec"`
	Health          HealthConfig   `mapstructure:"health"`
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

type ScheduleConfig struct {
	Timezone            string              `mapstructure:"timezone"`
	Workdays            []string            `mapstructure:"workdays"`
	BusinessHours       BusinessHoursConfig `mapstructure:"business_hours"`
	WeekendsAutoApprove bool                `mapstructure:"weekends_auto_approve"`
}

type BusinessHoursConfig struct {
	Start string `mapstructure:"start"`
	End   string `mapstructure:"end"`
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
	v.SetDefault("schedule.timezone", "Asia/Shanghai")
	v.SetDefault("schedule.workdays", []string{"monday", "tuesday", "wednesday", "thursday", "friday"})
	v.SetDefault("schedule.business_hours.start", "10:00")
	v.SetDefault("schedule.business_hours.end", "19:00")
	v.SetDefault("schedule.weekends_auto_approve", true)
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
	if strings.TrimSpace(c.Schedule.Timezone) == "" {
		return fmt.Errorf("schedule.timezone must not be empty")
	}
	if len(c.Schedule.Workdays) == 0 {
		return fmt.Errorf("schedule.workdays must not be empty")
	}
	if !isValidClock(c.Schedule.BusinessHours.Start) {
		return fmt.Errorf("schedule.business_hours.start must be in HH:MM format")
	}
	if !isValidClock(c.Schedule.BusinessHours.End) {
		return fmt.Errorf("schedule.business_hours.end must be in HH:MM format")
	}
	if c.Schedule.BusinessHours.Start == c.Schedule.BusinessHours.End {
		return fmt.Errorf("schedule.business_hours.start and end must not be equal")
	}
	for _, day := range c.Schedule.Workdays {
		if !isValidWeekday(day) {
			return fmt.Errorf("schedule.workdays contains invalid weekday: %s", day)
		}
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

func isValidClock(value string) bool {
	_, err := time.Parse("15:04", strings.TrimSpace(value))
	return err == nil
}

func isValidWeekday(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday":
		return true
	default:
		return false
	}
}
