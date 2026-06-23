package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// AgentAuth 校验分站（代理）密钥，并把代理身份伪装成 token+user 上下文，
// 以便完整复用现有 relay 引擎（Distribute / controller.Relay / 计费）。
//
// 关键点：这里不创建任何 users / tokens 行。计费由 IsAgentRequest 标记接管，
// 直接扣减 Agent.Balance（见 service.AgentFunding）。ContextKeyUserId 固定为 0，
// 避免与真实用户 id 撞号污染 users / logs 表。
func AgentAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		key := strings.TrimSpace(c.Request.Header.Get("Authorization"))
		if strings.HasPrefix(key, "Bearer ") || strings.HasPrefix(key, "bearer ") {
			key = strings.TrimSpace(key[7:])
		}
		key = strings.TrimPrefix(key, "sk-")
		if key == "" {
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "未提供分站密钥")
			return
		}

		agent, err := model.GetAgentByKey(key)
		if err != nil || agent == nil {
			abortWithOpenAiMessage(c, http.StatusUnauthorized, "分站密钥无效")
			return
		}
		if agent.Status != model.AgentStatusEnabled {
			abortWithOpenAiMessage(c, http.StatusForbidden, "分站已被禁用")
			return
		}
		// 注意：不在此处拒绝余额<=0。AgentAuth 同时用于 relay 与 /api/agent-self(查余额)，
		// 余额为 0 时仍需允许查询；relay 的余额保护由计费层预扣(AgentFunding.PreConsume)兜底。

		// ---- 伪装用户上下文（id 固定 0，防止撞号真实用户）----
		c.Set(string(constant.ContextKeyUserId), 0)
		common.SetContextKey(c, constant.ContextKeyUserGroup, agent.Group)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, agent.Group)
		common.SetContextKey(c, constant.ContextKeyUserQuota, clampToInt(agent.Balance))
		common.SetContextKey(c, constant.ContextKeyUserStatus, common.UserStatusEnabled)
		common.SetContextKey(c, constant.ContextKeyUserName, "agent:"+agent.Name)
		common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{})

		// ---- 伪装令牌上下文（无真实 token，余额由 Agent.Balance 兜底）----
		common.SetContextKey(c, constant.ContextKeyTokenId, 0)
		common.SetContextKey(c, constant.ContextKeyTokenKey, "")
		common.SetContextKey(c, constant.ContextKeyTokenUnlimited, true)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, agent.Group)
		if agent.ModelLimits != "" {
			c.Set(string(constant.ContextKeyTokenModelLimitEnabled), true)
			c.Set(string(constant.ContextKeyTokenModelLimit), agent.GetModelLimitsMap())
		} else {
			c.Set(string(constant.ContextKeyTokenModelLimitEnabled), false)
		}

		// ---- 代理标记，计费层据此走 AgentFunding ----
		common.SetContextKey(c, constant.ContextKeyIsAgentRequest, true)
		common.SetContextKey(c, constant.ContextKeyAgentId, agent.Id)

		c.Next()
	}
}

func clampToInt(v int64) int {
	const maxInt = int64(^uint(0) >> 1)
	if v > maxInt {
		return int(maxInt)
	}
	return int(v)
}
