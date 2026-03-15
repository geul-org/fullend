const BASE = '/api'

async function activateWorkflow(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/workflows/${id}/activate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function addAction(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/workflows/${id}/actions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function archiveWorkflow(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/workflows/${id}/archive`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function createWorkflow(params?: Record<string, any>) {
  const body = params ?? {}
  const res = await fetch(`${BASE}/workflows`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function executeWorkflow(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/workflows/${id}/execute`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function getWorkflow(params?: Record<string, any>) {
  const id = params?.id
  const query = new URLSearchParams()
  if (params) {
    const exclude = new Set(['id'])
    for (const [k, v] of Object.entries(params)) {
      if (v != null && !exclude.has(k)) query.set(k, String(v))
    }
  }
  const qs = query.toString()
  const res = await fetch(`${BASE}/workflows/${id}${qs ? '?' + qs : ''}`)
  return res.json()
}

async function listActions(params?: Record<string, any>) {
  const id = params?.id
  const query = new URLSearchParams()
  if (params) {
    const exclude = new Set(['id'])
    for (const [k, v] of Object.entries(params)) {
      if (v != null && !exclude.has(k)) query.set(k, String(v))
    }
  }
  const qs = query.toString()
  const res = await fetch(`${BASE}/workflows/${id}/actions${qs ? '?' + qs : ''}`)
  return res.json()
}

async function listWorkflows(params?: Record<string, any>) {
  const query = new URLSearchParams()
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v != null) query.set(k, String(v))
    }
  }
  const qs = query.toString()
  const res = await fetch(`${BASE}/workflows${qs ? '?' + qs : ''}`)
  return res.json()
}

async function login(params?: Record<string, any>) {
  const body = params ?? {}
  const res = await fetch(`${BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function pauseWorkflow(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/workflows/${id}/pause`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function register(params?: Record<string, any>) {
  const body = params ?? {}
  const res = await fetch(`${BASE}/auth/register`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

export const api = {
  ActivateWorkflow: activateWorkflow,
  AddAction: addAction,
  ArchiveWorkflow: archiveWorkflow,
  CreateWorkflow: createWorkflow,
  ExecuteWorkflow: executeWorkflow,
  GetWorkflow: getWorkflow,
  ListActions: listActions,
  ListWorkflows: listWorkflows,
  Login: login,
  PauseWorkflow: pauseWorkflow,
  Register: register
}
