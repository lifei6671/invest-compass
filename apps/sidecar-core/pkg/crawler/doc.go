// Package crawler 提供受控的外部数据抓取基础类库。
//
// 本包只负责 URL、Header、Query、超时、响应体大小、文本解码、JSON 解码和动态渲染扩展边界，
// 不承载具体行情解析逻辑，也不直接绑定 resty、chromedp、goquery、otto 等第三方依赖。
package crawler
