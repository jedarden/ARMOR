import React from 'react'
import { createRoot } from 'react-dom/client'
import { Agentation } from 'https://esm.sh/agentation@3.0.2?external=react,react-dom'

const mount = document.createElement('div')
mount.id = 'agentation-root'
document.body.appendChild(mount)
createRoot(mount).render(React.createElement(Agentation))
