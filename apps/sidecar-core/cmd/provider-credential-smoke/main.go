package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lifei6671/invest-compass/apps/sidecar-core/internal/dao"
	credentialservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/datasourcecredential"
	netproxyservice "github.com/lifei6671/invest-compass/apps/sidecar-core/internal/service/netproxy"
	gormlogger "gorm.io/gorm/logger"
)

type smokeOptions struct {
	Workspace            string `json:"workspace"`
	Providers            string `json:"providers"`
	AllowNetwork         bool   `json:"allowNetwork"`
	ConfirmProviderTerms bool   `json:"confirmProviderTerms"`
}

type smokeCheck struct {
	ProviderID     string   `json:"providerId"`
	ProviderName   string   `json:"providerName"`
	Target         string   `json:"target"`
	Status         string   `json:"status"`
	ResponseTimeMS int      `json:"responseTimeMs,omitempty"`
	TestedAt       string   `json:"testedAt,omitempty"`
	Messages       []string `json:"messages"`
}

type smokeOutput struct {
	NetworkEnabled bool         `json:"networkEnabled"`
	Checks         []smokeCheck `json:"checks"`
}

// main 执行数据源凭据真实连接 smoke，输出中禁止包含任何明文凭据。
func main() {
	options, err := parseSmokeOptions(os.Args[1:], os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	output, err := runSmoke(context.Background(), options)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoded, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
	if shouldExitUnsuccessful(output) {
		os.Exit(1)
	}
}

// parseSmokeOptions 解析 smoke 参数，联网模式必须显式确认第三方 Provider 条款和授权边界。
func parseSmokeOptions(args []string, getenv func(string) string) (smokeOptions, error) {
	flags := flag.NewFlagSet("provider-credential-smoke", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	options := smokeOptions{Providers: "all"}
	flags.StringVar(&options.Workspace, "workspace", "", "Invest Compass workspace path")
	flags.StringVar(&options.Providers, "providers", options.Providers, "all, configured, or comma-separated provider ids")
	flags.BoolVar(&options.AllowNetwork, "allow-network", false, "allow live third-party provider requests")
	flags.BoolVar(&options.ConfirmProviderTerms, "confirm-provider-terms", false, "confirm provider terms and authorization boundaries")
	if err := flags.Parse(stripArgumentSeparators(args)); err != nil {
		return smokeOptions{}, err
	}
	if options.AllowNetwork && !options.ConfirmProviderTerms {
		return smokeOptions{}, fmt.Errorf("--confirm-provider-terms is required when --allow-network is set")
	}
	if strings.TrimSpace(options.Workspace) == "" {
		options.Workspace = strings.TrimSpace(getenv("INVEST_COMPASS_WORKSPACE"))
	}
	if strings.TrimSpace(options.Workspace) == "" {
		home := strings.TrimSpace(getenv("HOME"))
		if home == "" {
			if detectedHome, err := os.UserHomeDir(); err == nil {
				home = detectedHome
			}
		}
		if home != "" {
			options.Workspace = filepath.Join(home, "Documents", "Invest Compass")
		}
	}
	if strings.TrimSpace(options.Workspace) == "" {
		return smokeOptions{}, fmt.Errorf("workspace is required")
	}
	options.Workspace = strings.TrimSpace(options.Workspace)
	options.Providers = strings.TrimSpace(options.Providers)
	if options.Providers == "" {
		options.Providers = "all"
	}
	return options, nil
}

// stripArgumentSeparators 移除 pnpm/npm 透传参数时常见的 "--" 分隔符。
func stripArgumentSeparators(args []string) []string {
	result := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--" {
			continue
		}
		result = append(result, arg)
	}
	return result
}

// runSmoke 复用生产 DAO、凭据 service 和动态代理 HTTP client 执行真实连接测试。
func runSmoke(ctx context.Context, options smokeOptions) (smokeOutput, error) {
	db, err := dao.Open(ctx, dao.Config{Path: filepath.Join(options.Workspace, "invest-compass.sqlite3")})
	if err != nil {
		return smokeOutput{}, err
	}
	db.Logger = gormlogger.Default.LogMode(gormlogger.Silent)
	sqlDB, err := db.DB()
	if err != nil {
		return smokeOutput{}, err
	}
	defer sqlDB.Close()

	store, err := dao.NewStore(db)
	if err != nil {
		return smokeOutput{}, err
	}
	service := credentialservice.NewService(store, credentialservice.FileKeyProvider{
		Path: filepath.Join(options.Workspace, "credentials", "data-source.key"),
	})
	service.HTTPClient = netproxyservice.DynamicClientForSettings(store, 15*time.Second)

	view, err := service.List(ctx)
	if err != nil {
		return smokeOutput{}, err
	}
	providerIDs, err := selectProviderIDs(view, options.Providers)
	if err != nil {
		return smokeOutput{}, err
	}

	output := smokeOutput{NetworkEnabled: options.AllowNetwork}
	for _, providerID := range providerIDs {
		config := view.Configs[providerID]
		check := smokeCheck{
			ProviderID:   providerID,
			ProviderName: config.ProviderName,
			Target:       defaultTargetForProvider(providerID),
		}
		if !options.AllowNetwork {
			check.Status = "skipped"
			check.Messages = []string{"network smoke disabled"}
			output.Checks = append(output.Checks, check)
			continue
		}
		result, err := service.Test(ctx, credentialservice.TestRequest{ProviderID: providerID, Target: check.Target})
		if err != nil {
			check.Status = "failed"
			check.Messages = []string{err.Error()}
			output.Checks = append(output.Checks, check)
			continue
		}
		check.Status = result.Status
		check.ResponseTimeMS = result.ResponseTimeMS
		check.TestedAt = result.TestedAt
		check.Messages = result.Messages
		output.Checks = append(output.Checks, check)
	}
	return output, nil
}

// shouldExitUnsuccessful 判断 smoke 输出是否应让 CLI 返回非 0；联网模式下 untested 也不是通过。
func shouldExitUnsuccessful(output smokeOutput) bool {
	for _, check := range output.Checks {
		switch check.Status {
		case "success", "limited":
			continue
		case "skipped":
			if !output.NetworkEnabled {
				continue
			}
			return true
		default:
			return true
		}
	}
	return false
}

// selectProviderIDs 根据命令参数选择要测试的 Provider，保持后端目录顺序。
func selectProviderIDs(view credentialservice.ListView, mode string) ([]string, error) {
	trimmedMode := strings.TrimSpace(mode)
	if trimmedMode == "" || trimmedMode == "all" {
		result := make([]string, 0, len(view.Providers))
		for _, provider := range view.Providers {
			result = append(result, provider.ID)
		}
		return result, nil
	}
	if trimmedMode == "configured" {
		result := make([]string, 0, len(view.Providers))
		for _, provider := range view.Providers {
			if provider.AuthType == credentialservice.AuthTypeNone || provider.Status == credentialservice.StatusNormal || provider.Status == credentialservice.StatusExpiring {
				result = append(result, provider.ID)
			}
		}
		return result, nil
	}

	known := make(map[string]struct{}, len(view.Providers))
	for _, provider := range view.Providers {
		known[provider.ID] = struct{}{}
	}
	result := []string{}
	for _, item := range strings.Split(trimmedMode, ",") {
		providerID := strings.TrimSpace(item)
		if providerID == "" {
			continue
		}
		if _, ok := known[providerID]; !ok {
			return nil, fmt.Errorf("unknown provider: %s", providerID)
		}
		result = append(result, providerID)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("provider list is empty")
	}
	return result, nil
}

// joinProviderIDs 仅供测试断言稳定输出顺序。
func joinProviderIDs(providerIDs []string) string {
	return strings.Join(providerIDs, ",")
}

// defaultTargetForProvider 返回每个 Provider 的最小真实连接测试目标。
func defaultTargetForProvider(providerID string) string {
	switch providerID {
	case "tencent":
		return "kline"
	case "eastmoney":
		return "security_list"
	case "sina":
		return "quote"
	case "alpha-vantage":
		return "quote"
	case "cls":
		return "flash"
	case "xueqiu":
		return "hot_stock"
	case "akshare":
		return "connectivity"
	default:
		return "connectivity"
	}
}
