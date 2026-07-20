package config

import "time"

const defaultLogLevel = "info"
const defaultPort = 8080
const defaultAdminPort = 9090
const defaultShutdownTimeout = 30
const defaultValkeyAddr = "localhost:6379"
const defaultValkeyTTL = 3600
const defaultMaskCacheTTL = 3600
const defaultMaxDBConns = 25
const defaultMinDBConns = 1
const defaultMaxDBConnLifetimeMinutes = 30
const defaultShieldActionOnSuspicious = "mask"
const defaultOtelEndpoint = "localhost:4317"
const defaultOtelServiceName = "maskchain-gateway"
const defaultOtelEnvironment = "development"
const defaultOtelSamplingRatio = 1.0
const defaultEgressMaxIdleConns = 10
const defaultEgressIdleTimeout = 30 * time.Second
const defaultEgressMaxRetries = 3
const defaultEgressBaseBackoff = 100 * time.Millisecond
const defaultEgressRetryOn5xx = false
const defaultEgressMaxIdleConnsPerHost = 2
const defaultEgressDisableKeepAlives = false
const defaultEgressCBMaxFailures = 3
const defaultEgressCBCooldown = 30 * time.Second
const defaultDebugEnabled = false
const defaultDebugAdminToken = ""
const defaultRateLimitRate = 100
const defaultRateLimitWindowSec = 60
const defaultDictionaryCacheValkeyTTL = 300
const defaultDictionaryCacheLRUSize = 10000
const defaultDictionaryCacheWarmOnStartup = true
const defaultDictionaryCacheWarmConcurrency = 5
const defaultSessionTTL = 30 * time.Minute
const defaultSessionMaxTTL = 24 * time.Hour
const defaultSessionCleanupInterval = 5 * time.Minute
const defaultSessionCleanupEnabled = false
const defaultSessionCacheTTL = 5 * time.Minute
const defaultAnalyticsRetentionDays = 90
const defaultAnalyticsBatchInterval = "5s"
const defaultHealthCheckCriticalDeps = "database"
const defaultTenantReloadInterval = 15 * time.Second
const defaultAdminSessionTTL = 30 * time.Minute
const defaultDashboardPollInterval = 5 * time.Second

// @sk-task 10-gateway-skeleton#T1.2: Set ServerConfig defaults in DefaultConfig (AC-001, AC-005)
//
// DefaultConfig handles the operation.
func DefaultConfig() *Config {
	return &Config{
		Log: &LogConfig{
			Level: defaultLogLevel,
		},
		Server: &ServerConfig{
			Port:                 defaultPort,
			AdminPort:            defaultAdminPort,
			ShutdownTimeout:      defaultShutdownTimeout,
			TenantReloadInterval: defaultTenantReloadInterval,
			HealthCheck: &HealthCheckConfig{
				CriticalDeps: []string{defaultHealthCheckCriticalDeps},
			},
		},
		DB: &DatabaseConfig{
			MaxConns:        defaultMaxDBConns,
			MinConns:        defaultMinDBConns,
			MaxConnLifetime: time.Duration(defaultMaxDBConnLifetimeMinutes) * time.Minute,
		},
		Valkey: &ValkeyConfig{
			Addr:   defaultValkeyAddr,
			TTLSec: defaultValkeyTTL,
		},
		Mask: &MaskConfig{
			CacheTTLSec: defaultMaskCacheTTL,
		},
		Shield: &ShieldConfig{
			ActionOnSuspicious: defaultShieldActionOnSuspicious,
		},
		OTel: &OtelConfig{
			Endpoint:      defaultOtelEndpoint,
			ServiceName:   defaultOtelServiceName,
			Environment:   defaultOtelEnvironment,
			SamplingRatio: defaultOtelSamplingRatio,
		},
		Egress: &EgressConfig{
			MaxIdleConns:        defaultEgressMaxIdleConns,
			IdleTimeout:         defaultEgressIdleTimeout,
			MaxRetries:          defaultEgressMaxRetries,
			BaseBackoff:         defaultEgressBaseBackoff,
			RetryOn5xx:          defaultEgressRetryOn5xx,
			MaxIdleConnsPerHost: defaultEgressMaxIdleConnsPerHost,
			DisableKeepAlives:   defaultEgressDisableKeepAlives,
			CircuitBreaker: &CircuitBreakerConfig{
				MaxFailures: defaultEgressCBMaxFailures,
				Cooldown:    defaultEgressCBCooldown,
			},
		},
		Debug: &DebugConfig{
			Enabled:    defaultDebugEnabled,
			AdminToken: defaultDebugAdminToken,
		},
		Session: &SessionConfig{
			DefaultTTL:      defaultSessionTTL,
			MaxTTL:          defaultSessionMaxTTL,
			CleanupInterval: defaultSessionCleanupInterval,
			CleanupEnabled:  defaultSessionCleanupEnabled,
			CacheTTL:        defaultSessionCacheTTL,
		},
		DictionaryCache: &DictionaryCacheConfig{
			ValkeyTTLSec:    defaultDictionaryCacheValkeyTTL,
			LRUSize:         defaultDictionaryCacheLRUSize,
			WarmOnStartup:   defaultDictionaryCacheWarmOnStartup,
			WarmConcurrency: defaultDictionaryCacheWarmConcurrency,
		},
		Analytics: &AnalyticsConfig{
			RetentionDays: defaultAnalyticsRetentionDays,
			BatchInterval: defaultAnalyticsBatchInterval,
		},
		Admin: &AdminConfig{
			SessionTTL:            defaultAdminSessionTTL,
			DashboardPollInterval: defaultDashboardPollInterval,
		},
	}
}
