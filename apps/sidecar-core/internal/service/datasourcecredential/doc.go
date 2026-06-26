// Package datasourcecredential 管理数据源凭据的配置、脱敏、加密和真实连接测试。
//
// 真实凭据只在保存、解密和连接测试的受控调用内短暂进入内存；列表和详情接口只能返回
// 脱敏后的展示字段，连接测试也只持久化状态码、耗时和脱敏说明。
package datasourcecredential
