/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useCallback, useEffect, useState } from 'react'
import { Copy, Plus, RefreshCw } from 'lucide-react'
import { toast } from 'sonner'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  createAgent,
  deleteAgent,
  getAgentLogs,
  getAgents,
  topupAgent,
  updateAgent,
  type Agent,
  type AgentFormData,
  type AgentLog,
} from './api'

// 与后端 common.QuotaPerUnit 保持一致：quota ↔ 美元换算。
const QUOTA_PER_UNIT = 500000

const usd = (quota: number) => `$${(quota / QUOTA_PER_UNIT).toFixed(2)}`

const emptyForm: AgentFormData = {
  name: '',
  group: 'default',
  domain: '',
  model_limits: '',
  remark: '',
}

export function Agents() {
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(false)

  // 新建/编辑表单
  const [formOpen, setFormOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form, setForm] = useState<AgentFormData>(emptyForm)
  const [saving, setSaving] = useState(false)

  // 新建成功后一次性展示 AgentKey
  const [newKey, setNewKey] = useState<{ name: string; key: string } | null>(
    null
  )

  // 充值
  const [topupAgentTarget, setTopupAgentTarget] = useState<Agent | null>(null)
  const [topupUSD, setTopupUSD] = useState('')

  // 流水
  const [logsAgent, setLogsAgent] = useState<Agent | null>(null)
  const [logs, setLogs] = useState<AgentLog[]>([])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const res = await getAgents(1, 100)
      if (res.success) setAgents(res.data?.items ?? [])
      else toast.error(res.message || '加载失败')
    } catch {
      toast.error('加载分站列表失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const openCreate = () => {
    setEditingId(null)
    setForm(emptyForm)
    setFormOpen(true)
  }

  const openEdit = (a: Agent) => {
    setEditingId(a.id)
    setForm({
      name: a.name,
      group: a.group,
      domain: a.domain,
      model_limits: a.model_limits,
      remark: a.remark,
      status: a.status,
    })
    setFormOpen(true)
  }

  const saveForm = async () => {
    if (!form.name.trim()) {
      toast.error('分站名称不能为空')
      return
    }
    setSaving(true)
    try {
      if (editingId == null) {
        const res = await createAgent({ ...form, group: form.group || 'default' })
        if (res.success && res.data) {
          setFormOpen(false)
          setNewKey({ name: res.data.name, key: res.data.agent_key })
          await load()
        } else {
          toast.error(res.message || '创建失败')
        }
      } else {
        const res = await updateAgent({ ...form, id: editingId })
        if (res.success) {
          toast.success('已保存')
          setFormOpen(false)
          await load()
        } else {
          toast.error(res.message || '保存失败')
        }
      }
    } catch {
      toast.error('操作失败')
    } finally {
      setSaving(false)
    }
  }

  const toggleStatus = async (a: Agent) => {
    const next = a.status === 1 ? 2 : 1
    const res = await updateAgent({
      id: a.id,
      name: a.name,
      group: a.group,
      domain: a.domain,
      model_limits: a.model_limits,
      remark: a.remark,
      status: next,
    })
    if (res.success) {
      toast.success(next === 1 ? '已启用' : '已停用')
      load()
    } else toast.error(res.message || '操作失败')
  }

  const removeAgent = async (a: Agent) => {
    if (!window.confirm(`确定删除分站「${a.name}」？此操作不可恢复。`)) return
    const res = await deleteAgent(a.id)
    if (res.success) {
      toast.success('已删除')
      load()
    } else toast.error(res.message || '删除失败')
  }

  const doTopup = async () => {
    if (!topupAgentTarget) return
    const amount = parseFloat(topupUSD)
    if (!amount || Number.isNaN(amount)) {
      toast.error('请输入有效金额（美元，可负数扣减）')
      return
    }
    const quota = Math.round(amount * QUOTA_PER_UNIT)
    const res = await topupAgent(topupAgentTarget.id, quota)
    if (res.success) {
      toast.success('批发额度已调整')
      setTopupAgentTarget(null)
      setTopupUSD('')
      load()
    } else toast.error(res.message || '操作失败')
  }

  const openLogs = async (a: Agent) => {
    setLogsAgent(a)
    setLogs([])
    const res = await getAgentLogs(a.id, 1, 50)
    if (res.success) setLogs(res.data?.items ?? [])
    else toast.error(res.message || '加载流水失败')
  }

  const copy = (text: string) => {
    navigator.clipboard.writeText(text)
    toast.success('已复制')
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>代理 / 分站</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button variant='outline' size='sm' onClick={() => load()}>
            <RefreshCw className='size-4' />
            刷新
          </Button>
          <Button size='sm' onClick={openCreate}>
            <Plus className='size-4' />
            新建分站
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='rounded-md border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>ID</TableHead>
                  <TableHead>名称 / 域名</TableHead>
                  <TableHead>分组</TableHead>
                  <TableHead>批发余额</TableHead>
                  <TableHead>已用</TableHead>
                  <TableHead>请求数</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead className='text-right'>操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {agents.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={8}
                      className='text-muted-foreground text-center'
                    >
                      {loading ? '加载中…' : '暂无分站，点右上角「新建分站」'}
                    </TableCell>
                  </TableRow>
                ) : (
                  agents.map((a) => (
                    <TableRow key={a.id}>
                      <TableCell>{a.id}</TableCell>
                      <TableCell>
                        <div className='font-medium'>{a.name}</div>
                        <div className='text-muted-foreground text-xs'>
                          {a.domain || '—'}
                        </div>
                      </TableCell>
                      <TableCell>{a.group}</TableCell>
                      <TableCell>{usd(a.balance)}</TableCell>
                      <TableCell>{usd(a.used_quota)}</TableCell>
                      <TableCell>{a.request_count}</TableCell>
                      <TableCell>
                        {a.status === 1 ? (
                          <Badge>启用</Badge>
                        ) : (
                          <Badge variant='destructive'>停用</Badge>
                        )}
                      </TableCell>
                      <TableCell className='space-x-1 text-right'>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => {
                            setTopupAgentTarget(a)
                            setTopupUSD('')
                          }}
                        >
                          充值
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => openEdit(a)}
                        >
                          编辑
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => openLogs(a)}
                        >
                          流水
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => toggleStatus(a)}
                        >
                          {a.status === 1 ? '停用' : '启用'}
                        </Button>
                        <Button
                          variant='destructive'
                          size='sm'
                          onClick={() => removeAgent(a)}
                        >
                          删除
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      {/* 新建 / 编辑 */}
      <Dialog open={formOpen} onOpenChange={setFormOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {editingId == null ? '新建分站' : `编辑分站 #${editingId}`}
            </DialogTitle>
            <DialogDescription>
              分组(group)决定该分站可用的渠道与批发价；新建后会生成 AgentKey
              供分站程序对接。
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-3'>
            <div className='space-y-1'>
              <Label>分站名称</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder='小明的 AI 中转站'
              />
            </div>
            <div className='space-y-1'>
              <Label>批发分组 group</Label>
              <Input
                value={form.group}
                onChange={(e) => setForm({ ...form, group: e.target.value })}
                placeholder='default'
              />
            </div>
            <div className='space-y-1'>
              <Label>域名（备注用，可选）</Label>
              <Input
                value={form.domain}
                onChange={(e) => setForm({ ...form, domain: e.target.value })}
                placeholder='ai.xiaoming.com'
              />
            </div>
            <div className='space-y-1'>
              <Label>模型限制（逗号分隔，留空=不限）</Label>
              <Input
                value={form.model_limits}
                onChange={(e) =>
                  setForm({ ...form, model_limits: e.target.value })
                }
                placeholder='gpt-4o-mini,gpt-4o'
              />
            </div>
            <div className='space-y-1'>
              <Label>备注（可选）</Label>
              <Input
                value={form.remark}
                onChange={(e) => setForm({ ...form, remark: e.target.value })}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setFormOpen(false)}>
              取消
            </Button>
            <Button onClick={saveForm} disabled={saving}>
              {saving ? '保存中…' : '保存'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 新建成功：一次性展示 AgentKey */}
      <Dialog open={!!newKey} onOpenChange={(o) => !o && setNewKey(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>分站「{newKey?.name}」已创建</DialogTitle>
            <DialogDescription>
              这是该分站的 AgentKey，请复制后粘贴到 TokenAgent 平台的「起分站」
              表单。关闭后仍可在列表查看。
            </DialogDescription>
          </DialogHeader>
          <div className='flex items-center gap-2 rounded-md border bg-muted p-3'>
            <code className='flex-1 break-all text-sm'>{newKey?.key}</code>
            <Button
              variant='outline'
              size='sm'
              onClick={() => newKey && copy(newKey.key)}
            >
              <Copy className='size-4' />
            </Button>
          </div>
          <DialogFooter>
            <Button onClick={() => setNewKey(null)}>完成</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 充值 */}
      <Dialog
        open={!!topupAgentTarget}
        onOpenChange={(o) => !o && setTopupAgentTarget(null)}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>调整批发额度</DialogTitle>
            <DialogDescription>
              分站「{topupAgentTarget?.name}」当前余额{' '}
              {topupAgentTarget ? usd(topupAgentTarget.balance) : ''}
              。输入正数充值、负数扣减（单位：美元）。
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-1'>
            <Label>金额（美元）</Label>
            <Input
              type='number'
              step='0.01'
              value={topupUSD}
              onChange={(e) => setTopupUSD(e.target.value)}
              placeholder='例如 10 或 -5'
            />
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => setTopupAgentTarget(null)}
            >
              取消
            </Button>
            <Button onClick={doTopup}>确认</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* 流水 */}
      <Dialog open={!!logsAgent} onOpenChange={(o) => !o && setLogsAgent(null)}>
        <DialogContent className='max-w-3xl'>
          <DialogHeader>
            <DialogTitle>分站「{logsAgent?.name}」消费流水</DialogTitle>
            <DialogDescription>最近 50 条批发消费记录。</DialogDescription>
          </DialogHeader>
          <div className='max-h-[60vh] overflow-auto rounded-md border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>时间</TableHead>
                  <TableHead>模型</TableHead>
                  <TableHead>提示</TableHead>
                  <TableHead>补全</TableHead>
                  <TableHead>批发扣费</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.length === 0 ? (
                  <TableRow>
                    <TableCell
                      colSpan={5}
                      className='text-muted-foreground text-center'
                    >
                      暂无流水
                    </TableCell>
                  </TableRow>
                ) : (
                  logs.map((l) => (
                    <TableRow key={l.id}>
                      <TableCell>
                        {new Date(l.created_at * 1000).toLocaleString()}
                      </TableCell>
                      <TableCell>{l.model_name}</TableCell>
                      <TableCell>{l.prompt_tokens}</TableCell>
                      <TableCell>{l.completion_tokens}</TableCell>
                      <TableCell>{usd(l.quota)}</TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>
          <DialogFooter>
            <Button onClick={() => setLogsAgent(null)}>关闭</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
