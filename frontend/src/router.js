import { createRouter, createWebHistory } from 'vue-router'
import { api } from './api'

const routes = [
  { path: '/', name: 'home', component: () => import('./views/HomeView.vue'), meta: { title: '首页' } },
  { path: '/topics/:id', name: 'topic', component: () => import('./views/TopicView.vue'), meta: { title: '主题' } },
  { path: '/categories/:id', name: 'category', component: () => import('./views/CategoryView.vue'), meta: { title: '分类' } },
  { path: '/videos/:id', name: 'watch', component: () => import('./views/WatchView.vue'), meta: { title: '播放' } },
  { path: '/search', name: 'search', component: () => import('./views/SearchView.vue'), meta: { title: '搜索' } },
  { path: '/materials', name: 'materials', component: () => import('./views/MaterialsHomeView.vue'), meta: { title: '资料' } },
  { path: '/materials/categories/:id', name: 'material-category', component: () => import('./views/MaterialCategoryView.vue'), meta: { title: '资料分类' } },
  { path: '/materials/search', name: 'material-search', component: () => import('./views/MaterialSearchView.vue'), meta: { title: '资料搜索' } },
  { path: '/materials/:id', name: 'material', component: () => import('./views/MaterialDetailView.vue'), meta: { title: '资料详情' } },
  { path: '/admin/login', name: 'admin-login', component: () => import('./views/admin/LoginView.vue'), meta: { title: '管理登录' } },
  { path: '/admin', name: 'admin', component: () => import('./views/admin/AdminView.vue'), meta: { title: '管理后台', admin: true } },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('./views/NotFoundView.vue'), meta: { title: '页面不存在' } }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior() {
    return { top: 0 }
  }
})

let sessionCheckedAt = 0
let sessionAdmin = false

router.beforeEach(async (to) => {
  document.title = to.meta.title ? `${to.meta.title} · StudyVideo` : 'StudyVideo'
  if (!to.meta.admin) return true
  const now = Date.now()
  if (now - sessionCheckedAt < 15000) {
    return sessionAdmin ? true : { name: 'admin-login', query: { redirect: to.fullPath } }
  }
  try {
    const res = await api.session()
    sessionAdmin = !!res.admin
  } catch {
    sessionAdmin = false
  }
  sessionCheckedAt = now
  if (!sessionAdmin) {
    return { name: 'admin-login', query: { redirect: to.fullPath } }
  }
  return true
})

export function invalidateSessionCache() {
  sessionCheckedAt = 0
  sessionAdmin = true
}

export default router
