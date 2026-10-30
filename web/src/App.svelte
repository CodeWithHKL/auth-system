<script lang="ts">
  import { onMount } from 'svelte'
  import { api, type User } from './lib/api'
  import AuthCard from './lib/AuthCard.svelte'
  import Profile from './lib/Profile.svelte'

  // undefined = still checking the session, null = signed out.
  let user = $state<User | null | undefined>(undefined)

  onMount(async () => {
    user = await api.me().catch(() => null)
  })

  async function logout() {
    await api.logout().catch(() => {})
    user = null
  }
</script>

<div class="bg" aria-hidden="true">
  <span class="orb o1"></span>
  <span class="orb o2"></span>
  <span class="orb o3"></span>
</div>

<main>
  {#if user === undefined}
    <div class="glass loading"><span class="spinner big" aria-label="Loading"></span></div>
  {:else if user}
    <Profile {user} onlogout={logout} />
  {:else}
    <AuthCard onauth={(u) => (user = u)} />
  {/if}
</main>
