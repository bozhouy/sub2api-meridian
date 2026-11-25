import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import CodexDiagnosticModal from '../CodexDiagnosticModal.vue'
import zh from '@/i18n/locales/zh/admin/accounts'
import type { Account } from '@/types'

const { fingerprint, ownKeys, diagnose } = vi.hoisted(() => ({ fingerprint: vi.fn(), ownKeys: vi.fn(), diagnose: vi.fn() }))
vi.mock('@/api/admin/codexDiagnostics', () => ({ fingerprint, refreshFingerprint: fingerprint, ownKeys, diagnose }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: ref('zh'), t: (key: string, values: Record<string, unknown> = {}) => {
  const text = key.split('.').reduce<any>((o, part) => o?.[part], { admin: zh })
  return typeof text === 'string' ? text.replace(/\{(\w+)\}/g, (_, k) => String(values[k])) : key
} }) }))

const account = { id: 3, name: 'Owner', platform: 'openai', type: 'oauth' } as Account
function create() {
  return mount(CodexDiagnosticModal, { props: { show: true, account }, global: { stubs: {
    BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
    Select: true, Icon: true
  } } })
}
beforeEach(() => {
  vi.resetAllMocks()
  fingerprint.mockResolvedValue({ commit: 'bank', models: ['gpt-5.6-terra', 'gpt-6-luna'] })
  ownKeys.mockResolvedValue([{ id: 1, name: 'Owner Key', status: 'active', quota: 0, quota_used: 0 }])
})

describe('account model diagnostic dialog', () => {
  it('starts only when the user clicks Start with a key and model selected', async () => {
    diagnose.mockResolvedValue({ items: [{ model: 'gpt-5.6-terra', status: 'normal' }] })
    const w = create(); await flushPromises()
    const vm = w.vm as any
    const start = w.get('[data-action="start"]')
    expect(start.attributes('disabled')).toBeDefined()
    vm.selectedKey = 1
    vm.selectedModels = ['gpt-5.6-terra']
    await flushPromises()
    expect(diagnose).not.toHaveBeenCalled()
    expect(start.attributes('disabled')).toBeUndefined()
    await start.trigger('click')
    await flushPromises()
    expect(diagnose).toHaveBeenCalledTimes(1)
    expect(diagnose).toHaveBeenCalledWith(3, 1, ['gpt-5.6-terra'], expect.any(AbortSignal))
    w.unmount()
  })
  it('uses the selected account, retains failed results and continues the queue', async () => {
    diagnose.mockResolvedValueOnce({ items: [{ model: 'gpt-5.6-terra', status: 'failed', reason: 'gateway_request_failed' }] })
      .mockResolvedValueOnce({ items: [{ model: 'gpt-6-luna', status: 'uncertain', reason: 'low_confidence' }] })
    const w = create(); await flushPromises()
    const vm = w.vm as any
    vm.selectedKey = 1; vm.selectedModels = ['gpt-5.6-terra', 'gpt-6-luna']
    await vm.runDiagnostic(); await flushPromises()
    expect(diagnose).toHaveBeenCalledWith(3, 1, ['gpt-5.6-terra'], expect.any(AbortSignal))
    expect(diagnose).toHaveBeenLastCalledWith(3, 1, ['gpt-6-luna'], expect.any(AbortSignal))
    expect(w.get('[data-model="gpt-5.6-terra"]').attributes('data-status')).toBe('failed')
    expect(w.text()).toContain('请求未完成，不代表降智')
    w.unmount()
  })
  it('cancels outstanding work when switching accounts and ignores stale results', async () => {
    let finish!: (value: unknown) => void
    diagnose.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const w = create(); await flushPromises()
    const vm = w.vm as any
    vm.selectedKey = 1; vm.selectedModels = ['gpt-5.6-terra', 'gpt-6-luna']
    const running = vm.runDiagnostic(); await flushPromises()
    const signal = diagnose.mock.calls[0][3] as AbortSignal
    await w.setProps({ account: { ...account, id: 4, name: 'Other' } })
    expect(signal.aborted).toBe(true)
    finish({ items: [{ model: 'gpt-5.6-terra', status: 'normal' }] }); await running; await flushPromises()
    expect(diagnose).toHaveBeenCalledTimes(1)
    expect(w.find('[data-model]').exists()).toBe(false)
    w.unmount()
  })
})
