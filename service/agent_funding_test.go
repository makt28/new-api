package service

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/stretchr/testify/require"
)

// 计费不变量：分站请求没有令牌额度（tokenConsumed 恒为 0），
// 若上游失败，已从 Agent.Balance 预扣的额度必须能被退还。
// 回归 needsRefundLocked 漏判 AgentFunding 预扣的缺陷（会导致失败请求漏扣分站批发额度）。
func TestBillingSessionNeedsRefundForAgentPreConsume(t *testing.T) {
	preConsumed := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{IsAgentRequest: true, AgentId: 1},
		funding:   &AgentFunding{agentId: 1, consumed: 96},
	}
	require.True(t, preConsumed.NeedsRefund(), "AgentFunding 已预扣时必须需要退款")

	notConsumed := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{IsAgentRequest: true, AgentId: 1},
		funding:   &AgentFunding{agentId: 1, consumed: 0},
	}
	require.False(t, notConsumed.NeedsRefund(), "未预扣时无需退款")

	settled := &BillingSession{
		relayInfo: &relaycommon.RelayInfo{IsAgentRequest: true, AgentId: 1},
		funding:   &AgentFunding{agentId: 1, consumed: 96},
		settled:   true,
	}
	require.False(t, settled.NeedsRefund(), "已结算后不得重复退款")
}
