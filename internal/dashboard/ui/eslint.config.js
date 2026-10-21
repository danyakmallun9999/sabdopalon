import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      globals: globals.browser,
    },
  },
  {
    files: ['**/*.{ts,tsx}'],
    rules: {
      // Vendored shadcn files intentionally co-locate non-component exports
      // with the component: cva variants next to their control (button,
      // badge) and the useSidebar hook inside the sidebar module it reads.
      // These names are the sanctioned exception; everything app-owned stays
      // in its own module.
      'react-refresh/only-export-components': [
        'error',
        {
          allowConstantExport: true,
          allowExportNames: ['badgeVariants', 'buttonVariants', 'useSidebar'],
        },
      ],
    },
  },
])
