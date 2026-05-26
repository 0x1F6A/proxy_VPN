import axios from 'axios'
import { useAuth } from '../store/auth'

export const http = axios.create({
  baseURL: '/api/v1',
  timeout: 15_000,
})

http.interceptors.request.use((config) => {
  const tok = useAuth.getState().token
  if (tok) config.headers.Authorization = `Bearer ${tok}`
  return config
})

http.interceptors.response.use(
  (resp) => {
    // Envelope: {code, message, data}
    const body = resp.data
    if (body && typeof body === 'object' && 'code' in body) {
      if (body.code !== 0) {
        return Promise.reject(new Error(body.message || `code=${body.code}`))
      }
      return { ...resp, data: body.data }
    }
    return resp
  },
  (err) => {
    if (err.response?.status === 401) {
      useAuth.getState().logout()
      if (location.pathname !== '/login') location.assign('/login')
    }
    const body = err.response?.data
    if (body?.message) return Promise.reject(new Error(body.message))
    return Promise.reject(err)
  },
)

export interface Page<T> {
  items?: T[]
  list?: T[]
  total: number
}
