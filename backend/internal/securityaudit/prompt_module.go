package securityaudit

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewPostgreSQLRepository,
	wire.Bind(new(JobRepository), new(*PostgreSQLRepository)),
	wire.Bind(new(EventRepository), new(*PostgreSQLRepository)),
	NewRedisPayloadStore,
	wire.Bind(new(PayloadStore), new(*RedisPayloadStore)),
	NewOpenAICompatibleScanner,
	wire.Bind(new(PromptScanner), new(*OpenAICompatibleScanner)),
	NewAtomicMetrics,
	wire.Bind(new(Metrics), new(*AtomicMetrics)),
	NewConfigManager,
	wire.Bind(new(ConfigStore), new(*ConfigManager)),
	NewPromptService,
	wire.Bind(new(PromptEngine), new(*PromptService)),
	// 上游 ProviderSet 漏了这条 Bind：其提交的 wire_gen.go 直接把 *PromptService
	// 传给 NewPromptAdminHandler，但 wire 重新生成时无法解析 PromptAdminService。
	// 补上后 `make generate` 可复现上游 wire_gen.go 的既有接线。
	wire.Bind(new(PromptAdminService), new(*PromptService)),
	NewLegacyModerationAdapter,
	NewCoordinator,
	NewPromptAdminHandler,
)
