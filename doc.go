/*
Package orm 提供独立的 MyBatis/MyBatis-Plus 风格 Go ORM 运行时。

包内包含实体元数据、Configuration、GlobalConfig/DbConfig、Statement 编译、SQL Provider、参数路径绑定、MetaObjectHandler 自动填充、ResultMap constructor/association/collection 嵌套映射和 discriminator 分派、SQLSession、执行器/Handler SPI、REUSE 预编译语句复用、事务 Session、BatchSession、游标流式查询、显式 Lazy 延迟加载、一级缓存、Mapper namespace 二级缓存、条件构造器、分页模型、主键策略、Service 层和拦截器 SPI。
生成器输出的 Mapper 只依赖 Session 接口，因此自动提交 Session、事务 Session、BatchSession 和流式查询签名可以共用同一套生成代码。
*/
package orm
