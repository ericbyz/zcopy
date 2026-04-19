import { createApp } from 'vue'
import App from './App.vue'
import router from './router'
import './style.css'
import zhCn from 'element-plus/es/locale/lang/zh-cn'

const app = createApp(App)
app.use(router)
app.provide('element-locale', zhCn)
app.mount('#app')
