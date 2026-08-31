package config

import (
	"log"
	"strings"
	"sync"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	App      AppConfig      `mapstructure:",squash"` // 与 config.yml 根级 name/env/log_* 及 APP_ENV 等对齐
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Session  SessionConfig  `mapstructure:"session"`
	LLM      LLMConfig      `mapstructure:"llm"`
	Pexels     PexelsConfig     `mapstructure:"pexels"`
	Iconify    IconifyConfig    `mapstructure:"iconify"`
	Mermaid    MermaidConfig    `mapstructure:"mermaid"`
	EmojiPack  EmojiPackConfig  `mapstructure:"emoji_pack"`
	SVGDiagram SVGDiagramConfig `mapstructure:"svg_diagram"`
	NanoBanana NanoBananaConfig `mapstructure:"nano_banana"`
	R2         R2Config         `mapstructure:"r2"`
}


type AppConfig struct {
	Name      string `mapstructure:"name"`
	Env       string `mapstructure:"env"`
	LogLevel  string `mapstructure:"log_level"`
	LogFormat string `mapstructure:"log_format"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DBName       string `mapstructure:"db_name"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type LLMConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

// PexelsConfig Pexels 图库检索。
type PexelsConfig struct {
	APIKey string `mapstructure:"api_key"`
}

type IconifyConfig struct {
	BaseURL   string `mapstructure:"base_url"`
	TimeoutMs int    `mapstructure:"timeout_ms"`
}

type MermaidConfig struct {
	CLI          string `mapstructure:"cli"`
	OutputFormat string `mapstructure:"output_format"`
	Theme        string `mapstructure:"theme"`
	Width        int    `mapstructure:"width"`
	Height       int    `mapstructure:"height"`
	TimeoutMs    int    `mapstructure:"timeout_ms"`
}

type EmojiPackConfig struct {
	Suffix    string `mapstructure:"suffix"`
	TimeoutMs int    `mapstructure:"timeout_ms"`
}

type SVGDiagramConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

type NanoBananaConfig struct {
	APIKey      string `mapstructure:"api_key"`
	Model       string `mapstructure:"model"`
	AspectRatio string `mapstructure:"aspect_ratio"`
}

// R2Config Cloudflare R2 应用配置（写入 objectstore.Options）。
// 密钥走 env：APP_R2_ACCESS_KEY_ID / APP_R2_SECRET_ACCESS_KEY
// 具体上传实现见 internal/pkg/objectstore，可复用于配图、头像等。
type R2Config struct {
	AccountID       string `mapstructure:"account_id"`
	AccessKeyID     string `mapstructure:"access_key_id"`
	SecretAccessKey string `mapstructure:"secret_access_key"`
	Bucket          string `mapstructure:"bucket"`
	Endpoint        string `mapstructure:"endpoint"`        // 可选
	PublicBaseURL   string `mapstructure:"public_base_url"` // 公开访问前缀
	KeyPrefix       string `mapstructure:"key_prefix"`      // 全局 key 前缀
}

func (c IconifyConfig) Normalized() IconifyConfig {
	out := c
	if out.BaseURL == "" {
		out.BaseURL = "https://api.iconify.design"
	}
	if out.TimeoutMs <= 0 {
		out.TimeoutMs = 5000
	}
	return out
}

func (c MermaidConfig) Normalized() MermaidConfig {
	out := c
	if out.CLI == "" {
		out.CLI = "mmdc"
	}
	if out.OutputFormat == "" {
		out.OutputFormat = "png"
	}
	if out.Theme == "" {
		out.Theme = "default"
	}
	if out.Width <= 0 {
		out.Width = 1200
	}
	if out.Height <= 0 {
		out.Height = 800
	}
	if out.TimeoutMs <= 0 {
		out.TimeoutMs = 30000
	}
	return out
}

func (c EmojiPackConfig) Normalized() EmojiPackConfig {
	out := c
	if out.Suffix == "" {
		out.Suffix = "表情包"
	}
	if out.TimeoutMs <= 0 {
		out.TimeoutMs = 8000
	}
	return out
}

func (c NanoBananaConfig) Normalized() NanoBananaConfig {
	out := c
	if out.Model == "" {
		out.Model = "gemini-2.0-flash-preview-image-generation"
	}
	if out.AspectRatio == "" {
		out.AspectRatio = "16:9"
	}
	return out
}

// Enabled 配置是否足以启用对象存储（与 objectstore.Options.Enabled 对齐）。
func (c R2Config) Enabled() bool {
	if c.AccessKeyID == "" || c.SecretAccessKey == "" || c.Bucket == "" {
		return false
	}
	if c.Endpoint == "" && c.AccountID == "" {
		return false
	}
	if c.PublicBaseURL == "" {
		return false
	}
	return true
}

// SessionConfig Cookie Session（Redis 后端）相关配置。
type SessionConfig struct {
	// Secret 用于签名 session cookie，生产必须通过环境变量覆盖。
	Secret string `mapstructure:"secret"`
	// MaxAge session 有效期（秒），默认 7 天。
	MaxAge int `mapstructure:"max_age"`
	// Secure 是否仅 HTTPS 发送 cookie；生产 HTTPS 环境应设为 true。
	Secure bool `mapstructure:"secure"`
}

const (
	defaultSessionSecret = "change-me-session-secret"
	defaultSessionMaxAge = 86400 * 7
)

// Normalized 返回填好默认值后的副本，避免调用方重复判断。
func (c SessionConfig) Normalized() SessionConfig {
	out := c
	if out.Secret == "" {
		out.Secret = defaultSessionSecret
	}
	if out.MaxAge <= 0 {
		out.MaxAge = defaultSessionMaxAge
	}
	return out
}

var (
	globalConfig *Config
	loadOnce     sync.Once
)

func LoadConfig() *Config {
	loadOnce.Do(func() {
		// 1. 本地开发时，从 .env 文件加载变量到操作系统的 Env 中。
		// 线上通过 Docker compose / K8s 注入时，如果没有 .env 文件也没关系，所以忽略错误。
		_ = godotenv.Load()

		v := viper.New()

		// 2. 设置骨架配置文件 config.yml
		v.SetConfigName("config")   // 文件名不带扩展名
		v.SetConfigType("yml")      // 明确文件类型
		v.AddConfigPath(".")        // 告诉 viper 在当前目录寻找
		v.AddConfigPath("./config") // 也可以添加多个查找路径

		// 3. 环境变量覆盖（前缀 APP_，嵌套用 _）
		// 例: APP_DATABASE_HOST -> database.host
		//     APP_LLM_API_KEY   -> llm.api_key
		//     APP_PEXELS_API_KEY -> pexels.api_key
		v.SetEnvPrefix("APP")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()

		// 4. 读取 config.yml
		if err := v.ReadInConfig(); err != nil {
			log.Printf("⚠️ 提示: 读取 config.yml 失败, 如果是在纯环境变量驱动的容器内可忽略. Err: %v", err)
		}

		// 5. 解析到结构体
		globalConfig = &Config{}
		if err := v.Unmarshal(globalConfig); err != nil {
			log.Fatalf("❌ 解析配置失败: %v", err)
		}

		// 6. session 默认值
		globalConfig.Session = globalConfig.Session.Normalized()
		globalConfig.Iconify = globalConfig.Iconify.Normalized()
		globalConfig.Mermaid = globalConfig.Mermaid.Normalized()
		globalConfig.EmojiPack = globalConfig.EmojiPack.Normalized()
		globalConfig.NanoBanana = globalConfig.NanoBanana.Normalized()
	})

	return globalConfig
}

func GetConfig() *Config {
	if globalConfig == nil {
		log.Fatal("❌ 错误: 必须先调用 LoadConfig()")
	}
	return globalConfig
}
