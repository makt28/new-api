package middleware

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// headerAgentGroup 是分站按请求指定批发分组的头。分站(可信中转)据自身档位/模型映射设置，
// 主站再校验该分组在代理可用集合内。注意：分站侧必须剥离客户端伪造的同名头，避免终端用户越权。
const headerAgentGroup = "New-Api-Group"

// agentGroupAllowed 判断请求想用的分组是否在代理可用集合内：
//   - 等于默认分组 agent.Group → 允许；
//   - 配了显式白名单 Groups → 仅白名单内允许（auto 需显式列入）；
//   - 未配白名单 → 与普通用户一致：全局可选分组 + auto。
func agentGroupAllowed(agent *model.Agent, g string) bool {
	if g == agent.Group {
		return true
	}
	if list := agent.GroupsList(); len(list) > 0 {
		for _, x := range list {
			if x == g {
				return true
			}
		}
		return false
	}
	if g == "auto" {
		return true
	}
	return service.GroupInUserUsableGroups(agent.Group, g)
}

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

		// ---- 选定使用分组：分站可通过 New-Api-Group 头按请求指定，校验在可用集合内；否则用默认 ----
		usingGroup := agent.Group
		if reqGroup := strings.TrimSpace(c.Request.Header.Get(headerAgentGroup)); reqGroup != "" {
			if !agentGroupAllowed(agent, reqGroup) {
				abortWithOpenAiMessage(c, http.StatusForbidden, "分组 "+reqGroup+" 不在该分站可用范围内")
				return
			}
			usingGroup = reqGroup
		}

		// ---- 伪装用户上下文（id 固定 0，防止撞号真实用户）----
		c.Set(string(constant.ContextKeyUserId), 0)
		common.SetContextKey(c, constant.ContextKeyUserGroup, agent.Group)
		common.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
		common.SetContextKey(c, constant.ContextKeyUserQuota, clampToInt(agent.Balance))
		common.SetContextKey(c, constant.ContextKeyUserStatus, common.UserStatusEnabled)
		common.SetContextKey(c, constant.ContextKeyUserName, "agent:"+agent.Name)
		common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{})

		// ---- 伪装令牌上下文（无真实 token，余额由 Agent.Balance 兜底）----
		common.SetContextKey(c, constant.ContextKeyTokenId, 0)
		common.SetContextKey(c, constant.ContextKeyTokenKey, "")
		common.SetContextKey(c, constant.ContextKeyTokenUnlimited, true)
		common.SetContextKey(c, constant.ContextKeyTokenGroup, usingGroup)
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

// AgentBalanceRequired 仅用于 relay 路径的批发额度门槛：余额<=0 直接拒绝，
// 避免免费/取整/分层导致预扣为 0 时仍把响应交付出去。
// 不放进 AgentAuth，是为了让 /api/agent-self 在零余额时仍可查询。
func AgentBalanceRequired() func(c *gin.Context) {
	return func(c *gin.Context) {
		agentId := common.GetContextKeyInt(c, constant.ContextKeyAgentId)
		agent, err := model.GetAgentById(agentId)
		if err != nil || agent == nil {
			abortWithOpenAiMessage(c, http.StatusForbidden, "分站不存在")
			return
		}
		if agent.Balance <= 0 {
			abortWithOpenAiMessage(c, http.StatusForbidden, "分站批发额度不足，请充值")
			return
		}
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
