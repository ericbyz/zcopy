import { createApp } from 'vue'
import App from './App.vue'
import 'element-plus/dist/index.css'
import './style.css'
import zhCn from 'element-plus/es/locale/lang/zh-cn'

const app = createApp(App)
app.provide('element-locale', zhCn)
app.mount('#app')
