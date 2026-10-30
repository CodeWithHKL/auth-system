<script lang="ts">
  import { api, ApiError, type User } from './api'

  let { onauth }: { onauth: (user: User) => void } = $props()

  type Mode = 'login' | 'signup'
  let mode = $state<Mode>('login')
  let name = $state('')
  let email = $state('')
  let password = $state('')
  let showPassword = $state(false)
  let busy = $state(false)
  let formError = $state('')
  let fieldErrors = $state<Record<string, string>>({})

  // Strength meter: one point each for length, mixed case, digit, symbol.
  const strength = $derived.by(() => {
    let s = 0
    if (password.length >= 8) s++
    if (/[a-z]/.test(password) && /[A-Z]/.test(password)) s++
    if (/\d/.test(password)) s++
    if (/[^A-Za-z0-9]/.test(password)) s++
    return password.length >= 12 && s >= 3 ? 4 : s
  })
  const strengthLabel = $derived(['Too weak', 'Weak', 'Fair', 'Good', 'Strong'][strength])

  function switchMode(next: Mode) {
    if (next === mode) return
    mode = next
    formError = ''
    fieldErrors = {}
  }

  function validate(): Record<string, string> {
    const errs: Record<string, string> = {}
    if (mode === 'signup' && !name.trim()) errs.name = 'Name is required'
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) errs.email = 'Enter a valid email address'
    if (!password) errs.password = 'Password is required'
    else if (mode === 'signup' && password.length < 8) errs.password = 'Use at least 8 characters'
    return errs
  }

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    formError = ''
    fieldErrors = validate()
    if (Object.keys(fieldErrors).length) return

    busy = true
    try {
      const user =
        mode === 'signup'
          ? await api.signup(name.trim(), email.trim(), password)
          : await api.login(email.trim(), password)
      onauth(user)
    } catch (err) {
      if (err instanceof ApiError) {
        fieldErrors = err.fields ?? {}
        formError = Object.keys(fieldErrors).length ? '' : err.message
      } else {
        formError = 'Cannot reach the server. Try again.'
      }
    } finally {
      busy = false
    }
  }

  // Cursor-follow sheen and a subtle 3D tilt on the card.
  let card: HTMLElement
  function onpointermove(e: PointerEvent) {
    const r = card.getBoundingClientRect()
    const x = (e.clientX - r.left) / r.width
    const y = (e.clientY - r.top) / r.height
    card.style.setProperty('--mx', `${x * 100}%`)
    card.style.setProperty('--my', `${y * 100}%`)
    card.style.setProperty('--rx', `${(0.5 - y) * 6}deg`)
    card.style.setProperty('--ry', `${(x - 0.5) * 8}deg`)
  }
  function onpointerleave() {
    card.style.setProperty('--rx', '0deg')
    card.style.setProperty('--ry', '0deg')
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions (decorative pointer effect only) -->
<section class="glass card" bind:this={card} {onpointermove} {onpointerleave}>
  <header>
    <h1>{mode === 'login' ? 'Welcome back' : 'Create account'}</h1>
    <p class="sub">
      {mode === 'login' ? 'Sign in to continue to your account.' : 'Join in a few seconds. No spam, ever.'}
    </p>
  </header>

  <div class="tabs" role="tablist" data-mode={mode}>
    <span class="thumb" aria-hidden="true"></span>
    <button role="tab" aria-selected={mode === 'login'} type="button" onclick={() => switchMode('login')}>
      Log in
    </button>
    <button role="tab" aria-selected={mode === 'signup'} type="button" onclick={() => switchMode('signup')}>
      Sign up
    </button>
  </div>

  <form onsubmit={submit} novalidate>
    {#if mode === 'signup'}
      <div class="field" class:invalid={fieldErrors.name}>
        <input id="name" type="text" placeholder=" " autocomplete="name" bind:value={name} />
        <label for="name">Full name</label>
        {#if fieldErrors.name}<small role="alert">{fieldErrors.name}</small>{/if}
      </div>
    {/if}

    <div class="field" class:invalid={fieldErrors.email}>
      <input id="email" type="email" placeholder=" " autocomplete="email" bind:value={email} />
      <label for="email">Email</label>
      {#if fieldErrors.email}<small role="alert">{fieldErrors.email}</small>{/if}
    </div>

    <div class="field" class:invalid={fieldErrors.password}>
      <input
        id="password"
        type={showPassword ? 'text' : 'password'}
        placeholder=" "
        autocomplete={mode === 'signup' ? 'new-password' : 'current-password'}
        bind:value={password}
      />
      <label for="password">Password</label>
      <button
        type="button"
        class="peek"
        aria-label={showPassword ? 'Hide password' : 'Show password'}
        onclick={() => (showPassword = !showPassword)}
      >
        {showPassword ? 'Hide' : 'Show'}
      </button>
      {#if fieldErrors.password}<small role="alert">{fieldErrors.password}</small>{/if}
    </div>

    {#if mode === 'signup' && password}
      <div class="meter" data-level={strength} aria-live="polite">
        <div class="bars"><i></i><i></i><i></i><i></i></div>
        <span>{strengthLabel}</span>
      </div>
    {/if}

    {#if formError}<p class="form-error" role="alert">{formError}</p>{/if}

    <button class="submit" type="submit" disabled={busy}>
      {#if busy}<span class="spinner" aria-hidden="true"></span>{/if}
      {mode === 'login' ? 'Log in' : 'Create account'}
    </button>
  </form>

  <p class="switch">
    {#if mode === 'login'}
      New here? <button type="button" onclick={() => switchMode('signup')}>Create an account</button>
    {:else}
      Already registered? <button type="button" onclick={() => switchMode('login')}>Log in</button>
    {/if}
  </p>
</section>
