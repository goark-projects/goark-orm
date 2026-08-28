/*
Package orm 提供独立、显式、可生成的 Go ORM 运行时。

根包是稳定公共 API 门面；实体元数据、Configuration、GlobalConfig/DbConfig、结构化错误层级、Statement 编译、SQL Provider、参数路径绑定、MetaObjectHandler 自动填充、ResultMap constructor/association/collection 嵌套映射和 discriminator 分派、SQLSession、执行器/Handler SPI、REUSE 预编译语句复用、事务 Session、BatchSession、游标流式查询、显式 Lazy 延迟加载、一级缓存、Mapper namespace 二级缓存、条件构造器、分页模型、主键策略、Service 层和拦截器 SPI 的实现位于 internal/runtime。
生成器输出的 Mapper 只依赖 Session 接口，因此自动提交 Session、事务 Session、BatchSession 和流式查询签名可以共用同一套生成代码。
*/
package orm
