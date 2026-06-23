package model

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"

	"gorm.io/gorm"
)

// Agent 代理（分站）账号。
//
// 设计原则：本子系统的所有状态完全独立，绝不复用 users / tokens / quota 等现有表。
// 分站程序携带 AgentKey 调用主站 /agentapi/v1，消费直接从 Agent.Balance（批发额度池）扣减。
// 出问题时可直接停用相关路由并 DROP TABLE agents, agent_logs 回滚，对主站现有逻辑零影响。
type Agent struct {
	Id           int            `json:"id"`
	Name         string         `json:"name" gorm:"type:varchar(64);index" validate:"max=64"`
	AgentKey     string         `json:"agent_key" gorm:"type:varchar(64);uniqueIndex"` // 分站调用主站用的密钥
	Domain       string         `json:"domain" gorm:"type:varchar(128)"`               // 分站域名（仅备注展示用）
	Status       int            `json:"status" gorm:"type:int;default:1"`              // 1 启用 2 禁用
	Balance      int64          `json:"balance" gorm:"type:bigint;default:0"`          // 批发额度池（quota 单位）
	UsedQuota    int64          `json:"used_quota" gorm:"type:bigint;default:0"`       // 累计消耗额度
	RequestCount int            `json:"request_count" gorm:"type:int;default:0"`       // 累计请求数
	Group        string         `json:"group" gorm:"type:varchar(64);default:'default'"` // 批发分组，决定可用渠道与价格
	ModelLimits  string         `json:"model_limits" gorm:"type:text"`                   // 允许模型，逗号分隔，空=不限制
	Remark       string         `json:"remark" gorm:"type:varchar(255)" validate:"max=255"`
	CreatedAt    int64          `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    int64          `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

const (
	AgentStatusEnabled  = 1
	AgentStatusDisabled = 2
)

// AgentLog 代理消费流水。独立于主站 logs 表，供分站对账使用。
type AgentLog struct {
	Id               int    `json:"id"`
	AgentId          int    `json:"agent_id" gorm:"index"`
	ModelName        string `json:"model_name" gorm:"index"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	Quota            int    `json:"quota"` // 本次批发扣费
	ChannelId        int    `json:"channel_id"`
	UseTimeSeconds   int    `json:"use_time_seconds"`
	IsStream         bool   `json:"is_stream"`
	Content          string `json:"content" gorm:"type:text"`
	CreatedAt        int64  `json:"created_at" gorm:"autoCreateTime;index"`
}

func (agent *Agent) GetModelLimitsMap() map[string]bool {
	limits := make(map[string]bool)
	if agent.ModelLimits == "" {
		return limits
	}
	for _, m := range strings.Split(agent.ModelLimits, ",") {
		m = strings.TrimSpace(m)
		if m != "" {
			limits[m] = true
		}
	}
	return limits
}

func GetAgentByKey(key string) (*Agent, error) {
	if key == "" {
		return nil, errors.New("agent key is empty")
	}
	var agent Agent
	err := DB.Where("agent_key = ?", key).First(&agent).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func GetAgentById(id int) (*Agent, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	var agent Agent
	err := DB.First(&agent, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &agent, nil
}

func GetAllAgents(startIdx int, num int) (agents []*Agent, total int64, err error) {
	err = DB.Model(&Agent{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = DB.Order("id desc").Limit(num).Offset(startIdx).Find(&agents).Error
	return agents, total, err
}

func (agent *Agent) Insert() error {
	if agent.AgentKey == "" {
		key, err := common.GenerateKey()
		if err != nil {
			return err
		}
		agent.AgentKey = key
	}
	return DB.Create(agent).Error
}

func (agent *Agent) Update() error {
	// 只更新可配置字段，余额变更走 Increase/Decrease 原子操作，避免覆盖并发扣费。
	return DB.Model(agent).Select("name", "domain", "status", "group", "model_limits", "remark").Updates(agent).Error
}

func DeleteAgentById(id int) error {
	if id == 0 {
		return errors.New("id is empty")
	}
	return DB.Delete(&Agent{}, "id = ?", id).Error
}

// DecreaseAgentBalance 原子扣减批发额度。余额不足时返回错误且不扣减。
func DecreaseAgentBalance(id int, quota int) error {
	if quota < 0 {
		return errors.New("quota 不能为负数")
	}
	if quota == 0 {
		return nil
	}
	result := DB.Model(&Agent{}).
		Where("id = ? AND balance >= ?", id, quota).
		Update("balance", gorm.Expr("balance - ?", quota))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("代理批发额度不足")
	}
	return nil
}

// IncreaseAgentBalance 原子增加批发额度（充值 / 退还预扣）。
func IncreaseAgentBalance(id int, quota int) error {
	if quota < 0 {
		return errors.New("quota 不能为负数")
	}
	if quota == 0 {
		return nil
	}
	return DB.Model(&Agent{}).
		Where("id = ?", id).
		Update("balance", gorm.Expr("balance + ?", quota)).Error
}

// UpdateAgentUsed 累加消耗统计，与扣费分离，统计失败不影响计费。
func UpdateAgentUsed(id int, quota int) {
	if id == 0 {
		return
	}
	err := DB.Model(&Agent{}).Where("id = ?", id).Updates(map[string]any{
		"used_quota":    gorm.Expr("used_quota + ?", quota),
		"request_count": gorm.Expr("request_count + 1"),
	}).Error
	if err != nil {
		common.SysLog("failed to update agent used quota: " + err.Error())
	}
}

func RecordAgentLog(log *AgentLog) {
	if log == nil || log.AgentId == 0 {
		return
	}
	if err := DB.Create(log).Error; err != nil {
		common.SysLog("failed to record agent log: " + err.Error())
	}
}

func GetAgentLogs(agentId int, startIdx int, num int) (logs []*AgentLog, total int64, err error) {
	tx := DB.Model(&AgentLog{})
	if agentId != 0 {
		tx = tx.Where("agent_id = ?", agentId)
	}
	err = tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	return logs, total, err
}
