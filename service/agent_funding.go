package service

import (
	"github.com/QuantumNous/new-api/model"
)

// AgentFunding 分站（代理）资金来源。资金完全独立于 users / 订阅，
// 直接扣减 / 退还 Agent.Balance（批发额度池）。
type AgentFunding struct {
	agentId  int
	consumed int // 实际预扣额度，供退款使用
}

func (a *AgentFunding) Source() string { return BillingSourceAgent }

func (a *AgentFunding) PreConsume(amount int) error {
	if amount <= 0 {
		return nil
	}
	if err := model.DecreaseAgentBalance(a.agentId, amount); err != nil {
		return err
	}
	a.consumed = amount
	return nil
}

func (a *AgentFunding) Settle(delta int) error {
	if delta == 0 {
		return nil
	}
	if delta > 0 {
		return model.DecreaseAgentBalance(a.agentId, delta)
	}
	return model.IncreaseAgentBalance(a.agentId, -delta)
}

func (a *AgentFunding) Refund() error {
	if a.consumed <= 0 {
		return nil
	}
	return model.IncreaseAgentBalance(a.agentId, a.consumed)
}
