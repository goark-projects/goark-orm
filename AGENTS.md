# Goark ORM Agent Notes

## Project Boundary

保持 ORM 模块定位：优先显式映射、可生成代码和清晰事务边界，避免反射重度运行时魔法。

- Module path: `goark.dev/orm`.
- Keep this repository Go-native: explicit APIs, deterministic setup, small runtime contracts, and testable adapters.
- Do not import Java/Spring runtime scanning, reflection-heavy proxy models, or broad framework behavior unless a written design explicitly justifies it.
- Use UTF-8 and LF for all generated files. Go comments must be concise standard Simplified Chinese.
- Behavior changes and bug fixes require focused Go tests; pure documentation or repository metadata changes should still run the applicable static checks.

## Project Skills

Use the project-local skills in `.claude/skills/` when the task matches their scope. This directory is the canonical cross-tool skill copy for Claude Code, OpenCode, Codex, and other agents that can read project instructions. A mirror exists in `.codex/skills/` for Codex-specific surfaces.

- Go 运行时、泛型、接口设计、标准库优先实现: `.claude/skills/golang-pro/SKILL.md`
- 上下文取消、goroutine 生命周期、背压与并发安全: `.claude/skills/go-concurrency-patterns/SKILL.md`
- Goark 模块边界、扩展点和运行时契约: `.claude/skills/architecture-patterns/SKILL.md`
- 跨包结构、公共抽象和演进路线设计: `.claude/skills/architecture-designer/SKILL.md`
- 公共 API、接口最小化和兼容性边界: `.claude/skills/api-and-interface-design/SKILL.md`
- README、ADR 和长期项目上下文记录: `.claude/skills/documentation-and-adrs/SKILL.md`
- 多文件功能的分阶段实现与验证: `.claude/skills/incremental-implementation/SKILL.md`
- 需求不明确或新能力落地前的规格澄清: `.claude/skills/spec-driven-development/SKILL.md`
- 错误码、错误包装和失败契约: `.claude/skills/error-handling-patterns/SKILL.md`
- 测试失败、运行异常和根因定位: `.claude/skills/systematic-debugging/SKILL.md`
- 保持行为不变的代码简化与去复杂化: `.claude/skills/code-simplification/SKILL.md`
- 行为变更、缺陷修复和 Go 单元测试: `.claude/skills/test-driven-development/SKILL.md`
- 代码审查、质量门禁和风险识别: `.claude/skills/code-review-and-quality/SKILL.md`
- 吞吐、延迟、分配和基准驱动优化: `.claude/skills/performance-engineer/SKILL.md`
- 日志、指标、追踪和可观测运行面: `.claude/skills/observability-and-instrumentation/SKILL.md`
- GitHub Actions 初始化和 CI 模板: `.claude/skills/github-actions-templates/SKILL.md`
- GitHub PR 检查失败诊断和修复: `.claude/skills/gh-fix-ci/SKILL.md`
- 配置结构、默认值、校验和启动失败语义: `.claude/skills/config-validate/SKILL.md`
- 凭据、TLS、输入校验和安全默认值: `.claude/skills/security-best-practices/SKILL.md`
