package audit

// Option 配置审计中间件行为。
type Option func(*options)

type options struct {
	recordQueries       bool
	recordErrors        bool
	ignoreRecorderError bool
	skip                func(Event) bool
}

func defaultOptions() options {
	return options{
		recordErrors: true,
	}
}

// WithQueryEvents 控制是否记录普通查询事件。
func WithQueryEvents(enabled bool) Option {
	return func(opts *options) {
		opts.recordQueries = enabled
	}
}

// WithErrorEvents 控制业务执行失败时是否仍记录审计事件。
func WithErrorEvents(enabled bool) Option {
	return func(opts *options) {
		opts.recordErrors = enabled
	}
}

// WithIgnoreRecorderError 控制 Recorder 失败时是否忽略该错误。
func WithIgnoreRecorderError(enabled bool) Option {
	return func(opts *options) {
		opts.ignoreRecorderError = enabled
	}
}

// WithSkipFunc 设置事件跳过函数，返回 true 表示不记录该事件。
func WithSkipFunc(skip func(Event) bool) Option {
	return func(opts *options) {
		opts.skip = skip
	}
}

func applyOptions(items []Option) options {
	opts := defaultOptions()
	for _, item := range items {
		if item != nil {
			item(&opts)
		}
	}
	return opts
}
