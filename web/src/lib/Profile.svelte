<script lang="ts">
  import type { User } from './api'

  let { user, onlogout }: { user: User; onlogout: () => void } = $props()

  const initials = $derived(
    user.name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((p) => p[0].toUpperCase())
      .join(''),
  )
  const since = $derived(
    new Date(user.createdAt).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' }),
  )
</script>

<section class="glass card profile">
  <div class="avatar" aria-hidden="true">{initials}</div>
  <h1>Hello, {user.name}</h1>
  <p class="sub">You're signed in.</p>

  <dl>
    <div><dt>Email</dt><dd>{user.email}</dd></div>
    <div><dt>Member since</dt><dd>{since}</dd></div>
  </dl>

  <button class="submit ghost" type="button" onclick={onlogout}>Log out</button>
</section>
