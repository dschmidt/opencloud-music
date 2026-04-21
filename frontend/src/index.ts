import '@opencloud-eu/extension-sdk/tailwind.css'
import {
  defineWebApplication,
  type AppMenuItemExtension,
  type ApplicationInformation,
  type Extension
} from '@opencloud-eu/web-pkg'
import { urlJoin } from '@opencloud-eu/web-client'
import { computed } from 'vue'
import Music from './views/Music.vue'

const appId = 'music'

export default defineWebApplication({
  setup() {
    const routes = [
      {
        name: `${appId}-index`,
        path: '/',
        component: Music,
        meta: {
          authContext: 'hybrid'
        }
      }
    ]

    const appInfo = {
      name: 'Music',
      id: appId,
      icon: 'music-2-line'
    } satisfies ApplicationInformation

    const menuItem: AppMenuItemExtension = {
      id: `app.${appInfo.id}.menuItem`,
      type: 'appMenuItem',
      label: () => appInfo.name,
      color: '#7b2cbf',
      icon: appInfo.icon,
      priority: 50,
      path: urlJoin(appInfo.id)
    }

    const extensions = computed<Extension[]>(() => [menuItem])

    return {
      appInfo,
      routes,
      extensions
    }
  }
})
