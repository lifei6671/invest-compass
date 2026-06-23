// Package datasourcecredential 管理数据源凭据的配置、脱敏、加密和本地预检。
//
// 本包不访问真实第三方 Provider，不执行网络连通性测试；真实凭据只在保存时进入
// 加密流程，列表和详情接口只能返回脱敏后的展示字段。
package datasourcecredential
