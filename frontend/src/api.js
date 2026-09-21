async function request(path, { method = 'GET', body, params } = {}) {
  let url = path
  if (params) {
    const qs = new URLSearchParams()
    for (const [k, v] of Object.entries(params)) {
      if (v !== undefined && v !== null && v !== '') qs.append(k, v)
    }
    const s = qs.toString()
    if (s) url += `?${s}`
  }
  const options = {
    method,
    credentials: 'same-origin',
    headers: {}
  }
  if (body !== undefined) {
    options.headers['Content-Type'] = 'application/json'
    options.body = JSON.stringify(body)
  }
  const res = await fetch(url, options)
  let data = null
  try {
    data = await res.json()
  } catch {
    data = null
  }
  if (!res.ok) {
    const err = new Error((data && data.error) || `请求失败（${res.status}）`)
    err.status = res.status
    throw err
  }
  return data
}

async function upload(path, formData, method = 'POST') {
  const res = await fetch(path, { method, body: formData, credentials: 'same-origin' })
  let data = null
  try {
    data = await res.json()
  } catch {
    data = null
  }
  if (!res.ok) {
    const err = new Error((data && data.error) || `上传失败（${res.status}）`)
    err.status = res.status
    throw err
  }
  return data
}

export const api = {
  home: () => request('/api/home'),
  topics: () => request('/api/topics'),
  topic: (id) => request(`/api/topics/${id}`),
  category: (id, order = '') => request(`/api/categories/${id}`, { params: { order } }),
  video: (id) => request(`/api/videos/${id}`),
  play: (id) => request(`/api/videos/${id}/play`),
  reportWatch: (id) => request(`/api/videos/${id}/watch`, { method: 'POST' }),
  pdfFileUrl: (id) => `/api/pdfs/${id}/file`,
  pdfDownloadUrl: (id) => `/api/pdfs/${id}/download`,
  search: (q) => request('/api/search', { params: { q } }),

  // 资料区（独立于视频区）
  materialHome: () => request('/api/materials/home'),
  materials: (params) => request('/api/materials', { params }),
  materialSearch: (q) => request('/api/materials/search', { params: { q } }),
  materialCategory: (id) => request(`/api/material-categories/${id}`),
  material: (id) => request(`/api/materials/${id}`),
  materialFileUrl: (id) => `/api/materials/${id}/file`,
  materialDownloadUrl: (id) => `/api/materials/${id}/download`,

  session: () => request('/api/admin/session'),
  login: (password) => request('/api/admin/login', { method: 'POST', body: { password } }),
  logout: () => request('/api/admin/logout', { method: 'POST' }),

  overview: () => request('/api/admin/overview'),
  adminTopics: () => request('/api/admin/topics'),
  createTopic: (body) => request('/api/admin/topics', { method: 'POST', body }),
  updateTopic: (id, body) => request(`/api/admin/topics/${id}`, { method: 'PUT', body }),
  deleteTopic: (id) => request(`/api/admin/topics/${id}`, { method: 'DELETE' }),

  adminCategories: (topicId) => request('/api/admin/categories', { params: { topic_id: topicId } }),
  createCategory: (body) => request('/api/admin/categories', { method: 'POST', body }),
  updateCategory: (id, body) => request(`/api/admin/categories/${id}`, { method: 'PUT', body }),
  deleteCategory: (id, mode = 'keep') => request(`/api/admin/categories/${id}`, { method: 'DELETE', params: { mode } }),

  adminVideos: (params) => request('/api/admin/videos', { params }),
  createVideo: (body) => request('/api/admin/videos', { method: 'POST', body }),
  updateVideo: (id, body) => request(`/api/admin/videos/${id}`, { method: 'PUT', body }),
  deleteVideo: (id) => request(`/api/admin/videos/${id}`, { method: 'DELETE' }),
  batchCreate: (body) => request('/api/admin/videos/batch', { method: 'POST', body }),
  checkVideo: (id) => request(`/api/admin/videos/${id}/check`, { method: 'POST' }),

  adminPdfs: (videoId) => request(`/api/admin/videos/${videoId}/pdfs`),
  uploadPdf: (videoId, formData) => upload(`/api/admin/videos/${videoId}/pdfs`, formData),
  updatePdf: (id, formData) => upload(`/api/admin/pdfs/${id}`, formData, 'PUT'),
  deletePdf: (id) => request(`/api/admin/pdfs/${id}`, { method: 'DELETE' }),

  checkStatus: () => request('/api/admin/check/status'),
  checkAll: () => request('/api/admin/check', { method: 'POST' }),

  blocked: () => request('/api/admin/blocked'),
  unblock: (ip) => request(`/api/admin/blocked/${encodeURIComponent(ip)}`, { method: 'DELETE' }),
  usage: (limit = 50) => request('/api/admin/usage', { params: { limit } }),
  config: () => request('/api/admin/config'),

  // 资料管理
  adminMaterialCategories: () => request('/api/admin/material-categories'),
  createMaterialCategory: (body) => request('/api/admin/material-categories', { method: 'POST', body }),
  updateMaterialCategory: (id, body) => request(`/api/admin/material-categories/${id}`, { method: 'PUT', body }),
  deleteMaterialCategory: (id, mode = '') =>
    request(`/api/admin/material-categories/${id}`, { method: 'DELETE', params: { mode } }),
  adminMaterials: (params) => request('/api/admin/materials', { params }),
  uploadMaterial: (formData) => upload('/api/admin/materials', formData),
  updateMaterial: (id, formData) => upload(`/api/admin/materials/${id}`, formData, 'PUT'),
  deleteMaterial: (id) => request(`/api/admin/materials/${id}`, { method: 'DELETE' })
}
