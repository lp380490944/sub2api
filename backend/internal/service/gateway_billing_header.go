package service

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ccVersionInBillingRe matches the semver part of cc_version (X.Y.Z).
var ccVersionInBillingRe = regexp.MustCompile(`cc_version=\d+\.\d+\.\d+`)

var ccVersionWithFingerprintInBillingRe = regexp.MustCompile(`cc_version=\d+\.\d+\.\d+\.[0-9a-fA-F]{3}\b`)

// effectiveBillingUserAgent 返回"最终会出现在出站 User-Agent 头上"的值，供 body 里的
// cc_version 同步使用。取值必须与 buildUpstreamRequest / buildCountTokensRequest 写头的
// 顺序严格一致：
//   - fork：账号指纹存在时，调用方会在 mimic 头之后再 ApplyFingerprint 压回 → 取指纹 UA
//     （按账号分桶版本 + 注册设备 OS/Arch）；上游此处无条件取 mimicUserAgent；
//   - OAuth mimicry 且无指纹：取调用方同一请求内取一次的 mimicUserAgent（= DefaultUserAgent()）；
//   - 其它：不同步（保留客户端自己的 billing 块）。
func effectiveBillingUserAgent(mimicUserAgent, tokenType string, mimicClaudeCode bool, fingerprint *Fingerprint) string {
	if fingerprint != nil && fingerprint.UserAgent != "" {
		return fingerprint.UserAgent
	}
	if tokenType == "oauth" && mimicClaudeCode {
		return mimicUserAgent
	}
	return ""
}

// syncBillingHeaderVersion rewrites cc_version in x-anthropic-billing-header
// system text blocks to match the version extracted from userAgent.
// Recompute any recognized fingerprint suffix because its input includes the version.
// Only touches system array blocks whose text starts with "x-anthropic-billing-header".
func syncBillingHeaderVersion(body []byte, userAgent string) []byte {
	version := ExtractCLIVersion(userAgent)
	if version == "" {
		return body
	}

	systemResult := gjson.GetBytes(body, "system")
	if !systemResult.Exists() || !systemResult.IsArray() {
		return body
	}

	replacement := "cc_version=" + version
	idx := 0
	systemResult.ForEach(func(_, item gjson.Result) bool {
		text := item.Get("text")
		if text.Exists() && text.Type == gjson.String &&
			strings.HasPrefix(text.String(), "x-anthropic-billing-header") {
			fingerprintedReplacement := replacement + "." + computeClaudeCodeFingerprint(body, version)
			newText := ccVersionWithFingerprintInBillingRe.ReplaceAllString(text.String(), fingerprintedReplacement)
			newText = ccVersionInBillingRe.ReplaceAllString(newText, replacement)
			if newText != text.String() {
				if updated, err := sjson.SetBytes(body, fmt.Sprintf("system.%d.text", idx), newText); err == nil {
					body = updated
				}
			}
		}
		idx++
		return true
	})

	return body
}
