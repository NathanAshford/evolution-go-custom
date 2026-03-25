/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_DEFAULT_API_URL: string
  readonly VITE_DEFAULT_WS_URL: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
