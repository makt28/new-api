package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// 分站（代理）管理接口，仅管理员可用。代理账号完全独立于普通用户体系。

func GetAllAgents(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	agents, total, err := model.GetAllAgents(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(agents)
	common.ApiSuccess(c, pageInfo)
}

func GetAgent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	agent, err := model.GetAgentById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, agent)
}

func CreateAgent(c *gin.Context) {
	var agent model.Agent
	if err := c.ShouldBindJSON(&agent); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if agent.Name == "" {
		common.ApiErrorMsg(c, "分站名称不能为空")
		return
	}
	if agent.Group == "" {
		agent.Group = "default"
	}
	if agent.Status == 0 {
		agent.Status = model.AgentStatusEnabled
	}
	// 安全：余额不允许在创建时直接写入，必须通过充值接口。
	agent.Balance = 0
	agent.UsedQuota = 0
	agent.RequestCount = 0
	agent.AgentKey = ""
	if err := agent.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, agent)
}

func UpdateAgent(c *gin.Context) {
	var agent model.Agent
	if err := c.ShouldBindJSON(&agent); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if agent.Id == 0 {
		common.ApiErrorMsg(c, "缺少分站 id")
		return
	}
	if agent.Group == "" {
		agent.Group = "default"
	}
	if err := agent.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	updated, err := model.GetAgentById(agent.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, updated)
}

func DeleteAgent(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if err := model.DeleteAgentById(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

type TopUpAgentRequest struct {
	Id    int `json:"id"`
	Quota int `json:"quota"` // 正数充值，负数扣减
}

func TopUpAgent(c *gin.Context) {
	var req TopUpAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "无效的参数")
		return
	}
	if req.Id == 0 || req.Quota == 0 {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	var err error
	if req.Quota > 0 {
		err = model.IncreaseAgentBalance(req.Id, req.Quota)
	} else {
		err = model.DecreaseAgentBalance(req.Id, -req.Quota)
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	agent, err := model.GetAgentById(req.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, agent)
}

func GetAgentLogs(c *gin.Context) {
	agentId, _ := strconv.Atoi(c.Query("agent_id"))
	pageInfo := common.GetPageQuery(c)
	logs, total, err := model.GetAgentLogs(agentId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

// GetAgentSelf 供分站程序自查批发余额/用量（用 AgentKey 调用，经 AgentAuth 鉴权）。
func GetAgentSelf(c *gin.Context) {
	agentId := common.GetContextKeyInt(c, constant.ContextKeyAgentId)
	if agentId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "无效的分站身份"})
		return
	}
	agent, err := model.GetAgentById(agentId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"id":            agent.Id,
		"name":          agent.Name,
		"balance":       agent.Balance,
		"used_quota":    agent.UsedQuota,
		"request_count": agent.RequestCount,
		"group":         agent.Group,
		"status":        agent.Status,
	})
}

// GetAgentSelfGroups 供分站程序拉取本代理可用的分组清单（用 AgentKey 调用，经 AgentAuth 鉴权）。
// 返回与 middleware.agentGroupAllowed 同一套可用集合，分站据此给管理员做分组下拉，
// 不再手填分组字符串（填错即 403）。ratio 为该代理使用此分组的批发倍率，供分站定零售加价参考。
func GetAgentSelfGroups(c *gin.Context) {
	agentId := common.GetContextKeyInt(c, constant.ContextKeyAgentId)
	if agentId == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "无效的分站身份"})
		return
	}
	agent, err := model.GetAgentById(agentId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	usable := service.GetAgentUsableGroups(agent.Group, agent.GroupsList())
	type groupInfo struct {
		Name      string  `json:"name"`
		Desc      string  `json:"desc"`
		Ratio     float64 `json:"ratio"`
		IsDefault bool    `json:"is_default"`
	}
	groups := make([]groupInfo, 0, len(usable))
	for name, desc := range usable {
		groups = append(groups, groupInfo{
			Name:      name,
			Desc:      desc,
			Ratio:     service.GetUserGroupRatio(agent.Group, name),
			IsDefault: name == agent.Group,
		})
	}
	common.ApiSuccess(c, groups)
}
