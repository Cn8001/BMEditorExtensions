import vue from '@vitejs/plugin-vue'
import { bmext } from './bmext/vite.mjs'

export default { plugins: [vue()], ...bmext({ name: 'ascdii' }) }
