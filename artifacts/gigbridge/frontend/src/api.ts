const BASE = '/api'

async function acceptProposal(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/proposals/${id}/accept`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function approveWork(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/gigs/${id}/approve`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function createGig(params?: Record<string, any>) {
  const body = params ?? {}
  const res = await fetch(`${BASE}/gigs`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function getGig(params?: Record<string, any>) {
  const id = params?.id
  const query = new URLSearchParams()
  if (params) {
    const exclude = new Set(['id'])
    for (const [k, v] of Object.entries(params)) {
      if (v != null && !exclude.has(k)) query.set(k, String(v))
    }
  }
  const qs = query.toString()
  const res = await fetch(`${BASE}/gigs/${id}${qs ? '?' + qs : ''}`)
  return res.json()
}

async function listGigs(params?: Record<string, any>) {
  const query = new URLSearchParams()
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (v != null) query.set(k, String(v))
    }
  }
  const qs = query.toString()
  const res = await fetch(`${BASE}/gigs${qs ? '?' + qs : ''}`)
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

async function publishGig(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/gigs/${id}/publish`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function raiseDispute(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/gigs/${id}/dispute`, {
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

async function rejectProposal(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/proposals/${id}/reject`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function submitProposal(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/gigs/${id}/proposals`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

async function submitWork(params?: Record<string, any>) {
  const id = params?.id
  const exclude = new Set(['id'])
  const body: Record<string, any> = {}
  if (params) {
    for (const [k, v] of Object.entries(params)) {
      if (!exclude.has(k)) body[k] = v
    }
  }
  const res = await fetch(`${BASE}/gigs/${id}/submit-work`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return res.json()
}

export const api = {
  AcceptProposal: acceptProposal,
  ApproveWork: approveWork,
  CreateGig: createGig,
  GetGig: getGig,
  ListGigs: listGigs,
  Login: login,
  PublishGig: publishGig,
  RaiseDispute: raiseDispute,
  Register: register,
  RejectProposal: rejectProposal,
  SubmitProposal: submitProposal,
  SubmitWork: submitWork
}
