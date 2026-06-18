# Go core database migrations

首版 SQLite schema 由 `internal/dao.Migrate` 通过 GORM `AutoMigrate`
创建和补齐。

这里保留迁移目录，后续一旦需要发布后的不可逆 schema 调整，应在本目录
新增版本化迁移说明或脚本，并在升级前补充备份和回滚验证。
