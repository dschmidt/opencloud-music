<template>
  <div
    class="ext:flex ext:h-full ext:items-start ext:justify-center ext:overflow-auto ext:p-8 ext:pt-24"
  >
    <div class="ext:w-full ext:max-w-xl ext:space-y-4">
      <h1 class="ext:text-2xl ext:font-bold" data-testid="music-title">OpenCloud Music</h1>

      <p class="ext:text-sm">
        The Subsonic-compatible endpoint is active. Configure a Subsonic client (Symfonium,
        play:Sub, Feishin, …) with this server's URL and an OpenCloud app token to start streaming
        your library.
      </p>

      <div
        class="ext:rounded ext:border ext:border-role-outline ext:p-4 ext:text-sm"
        data-testid="music-status"
      >
        <template v-if="loading">Checking backend connection…</template>
        <template v-else-if="error">
          <span class="ext:text-role-error">Backend unreachable:</span> {{ error }}
        </template>
        <template v-else-if="username">
          Connected to music backend as <strong>{{ username }}</strong>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useClientService } from '@opencloud-eu/web-pkg'

const { httpAuthenticated } = useClientService()

const loading = ref(true)
const error = ref<string | null>(null)
const username = ref<string | null>(null)

// Single round-trip check that the music backend is reachable behind
// the OpenCloud proxy and that the Bearer token we hold passes through
// to Graph. `tokenInfo` echoes back the username the token resolves
// to; we surface it so the user immediately sees whose library is
// being served.
interface TokenInfoResponse {
  'subsonic-response'?: {
    status?: string
    tokenInfo?: { username?: string }
    error?: { message?: string }
  }
}

onMounted(async () => {
  try {
    const { data } = await httpAuthenticated.get<TokenInfoResponse>(
      '/api/music/rest/tokenInfo?f=json'
    )
    const resp = data['subsonic-response']
    if (resp?.status === 'ok' && resp.tokenInfo?.username) {
      username.value = resp.tokenInfo.username
    } else {
      error.value = resp?.error?.message ?? 'unexpected response shape'
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
})
</script>
