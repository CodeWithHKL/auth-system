<script lang="ts">
  import { onMount } from 'svelte'

  type Todo = { id: number; text: string; done: boolean }

  let todos = $state<Todo[]>([])
  let text = $state('')
  let error = $state('')

  let remaining = $derived(todos.filter((t) => !t.done).length)

  async function api(path: string, init?: RequestInit) {
    const res = await fetch(`/api/todos${path}`, init)
    if (!res.ok) throw new Error(`${res.status} ${await res.text()}`)
    return res.status === 204 ? null : res.json()
  }

  async function load() {
    try {
      todos = await api('')
      error = ''
    } catch (e) {
      error = `Could not reach the API: ${(e as Error).message}`
    }
  }

  async function add(e: SubmitEvent) {
    e.preventDefault()
    if (!text.trim()) return
    const todo: Todo = await api('', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ text }),
    })
    todos.push(todo)
    text = ''
  }

  async function toggle(todo: Todo) {
    const updated: Todo = await api(`/${todo.id}`, { method: 'PATCH' })
    todo.done = updated.done
  }

  async function remove(todo: Todo) {
    await api(`/${todo.id}`, { method: 'DELETE' })
    todos = todos.filter((t) => t.id !== todo.id)
  }

  onMount(load)
</script>

<main>
  <h1>Todos</h1>

  <form onsubmit={add}>
    <input bind:value={text} placeholder="What needs doing?" />
    <button type="submit">Add</button>
  </form>

  {#if error}
    <p class="error">{error}</p>
  {/if}

  <ul>
    {#each todos as todo (todo.id)}
      <li class:done={todo.done}>
        <label>
          <input type="checkbox" checked={todo.done} onchange={() => toggle(todo)} />
          {todo.text}
        </label>
        <button onclick={() => remove(todo)} aria-label="Delete">✕</button>
      </li>
    {/each}
  </ul>

  <p>{remaining} remaining</p>
</main>

<style>
  main {
    max-width: 480px;
    margin: 0 auto;
    padding: 32px 20px;
    text-align: left;
  }
  form {
    display: flex;
    gap: 8px;
  }
  form input {
    flex: 1;
  }
  ul {
    list-style: none;
    padding: 0;
  }
  li {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px solid var(--border);
  }
  li.done label {
    text-decoration: line-through;
    opacity: 0.6;
  }
  .error {
    color: crimson;
  }
</style>
