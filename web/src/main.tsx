import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import { installExternalLinks } from './lib/desktop'
import { HOST } from './lib/host'
import './index.css'

if (HOST === 'desktop') installExternalLinks()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
