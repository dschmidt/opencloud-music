import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import Music from '../../src/views/Music.vue'

describe('Music view', () => {
  it('renders the title and placeholder copy', () => {
    const wrapper = mount(Music)
    expect(wrapper.find('[data-testid="music-title"]').text()).toBe('OpenCloud Music')
    expect(wrapper.text()).toContain('Subsonic-compatible endpoint')
  })
})
