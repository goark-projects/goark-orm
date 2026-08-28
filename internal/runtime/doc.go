/*
Package runtime 承载 goark-orm 的核心运行时实现。

该包属于模块内部边界，负责 SQL 编译、执行会话、事务、缓存、结果映射、条件构造、服务层、方言和拦截器链等生产路径。外部调用方应继续依赖 goark.dev/orm 根包门面，避免绑定内部实现布局。
*/
package runtime
