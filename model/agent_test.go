package model

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 校验分站批发额度池的核心计费不变量：
//   - 余额充足时原子扣减
//   - 余额不足时拒绝且不扣减（绝不透支）
//   - 退还正确恢复余额
func TestDecreaseAgentBalanceRejectsOverdraft(t *testing.T) {
	truncateTables(t)

	agent := &Agent{Name: "sub1", Group: "default", Status: AgentStatusEnabled, Balance: 100}
	require.NoError(t, agent.Insert())
	require.NotEmpty(t, agent.AgentKey, "Insert 应自动生成 AgentKey")

	// 扣减 60，余额应为 40
	require.NoError(t, DecreaseAgentBalance(agent.Id, 60))
	got, err := GetAgentById(agent.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(40), got.Balance)

	// 再扣 60 超出余额，应失败且余额不变
	err = DecreaseAgentBalance(agent.Id, 60)
	require.Error(t, err)
	got, err = GetAgentById(agent.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(40), got.Balance, "扣减失败时余额不得变动")

	// 退还 60，余额恢复为 100
	require.NoError(t, IncreaseAgentBalance(agent.Id, 60))
	got, err = GetAgentById(agent.Id)
	require.NoError(t, err)
	assert.Equal(t, int64(100), got.Balance)
}

// 并发扣减不得使余额变为负数：总扣减成功额不超过初始余额。
func TestDecreaseAgentBalanceConcurrentNoOverdraft(t *testing.T) {
	truncateTables(t)

	agent := &Agent{Name: "sub2", Group: "default", Status: AgentStatusEnabled, Balance: 100}
	require.NoError(t, agent.Insert())

	const workers = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 每次扣 10，最多应有 10 次成功（100/10）
			if err := DecreaseAgentBalance(agent.Id, 10); err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	got, err := GetAgentById(agent.Id)
	require.NoError(t, err)
	assert.Equal(t, 10, success, "恰好 10 次扣减成功")
	assert.Equal(t, int64(0), got.Balance, "余额恰好扣到 0，不透支")
	assert.GreaterOrEqual(t, got.Balance, int64(0))
}
