// Package search 承载本地范围搜索的分词、索引文本构造、排序和查询编排。
//
// 本包不注册 HTTP route，不直接访问 Rust command，也不绕过 dao 层直接打开数据库。
// 用户输入必须先经过归一化和 MATCH 构造器，不能把原始查询语法透传给 SQLite FTS5。
package search
