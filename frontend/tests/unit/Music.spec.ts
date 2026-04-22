import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const get = vi.fn()
vi.mock('@opencloud-eu/web-pkg', () => ({
  useClientService: () => ({ httpAuthenticated: { get } })
}))

import Music from '../../src/views/Music.vue'

describe('Music view', () => {
  it('renders the title and placeholder copy', async () => {
    get.mockResolvedValueOnce({ data: { 'subsonic-response': { status: 'ok' } } })
    const wrapper = mount(Music)
    await flushPromises()
    expect(wrapper.find('[data-testid="music-title"]').text()).toBe('OpenCloud Music')
    expect(wrapper.text()).toContain('Subsonic-compatible endpoint')
  })

  it('shows the username from tokenInfo when the backend answers', async () => {
    get.mockResolvedValueOnce({
      data: { 'subsonic-response': { status: 'ok', tokenInfo: { username: 'admin' } } }
    })
    const wrapper = mount(Music)
    await flushPromises()
    expect(get).toHaveBeenCalledWith('/api/music/rest/tokenInfo?f=json')
    expect(wrapper.find('[data-testid="music-status"]').text()).toContain('admin')
  })

  it('surfaces errors when the backend call fails', async () => {
    get.mockRejectedValueOnce(new Error('proxy down'))
    const wrapper = mount(Music)
    await flushPromises()
    const status = wrapper.find('[data-testid="music-status"]')
    expect(status.text()).toContain('Backend unreachable')
    expect(status.text()).toContain('proxy down')
  })
})
