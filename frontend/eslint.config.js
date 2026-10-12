import js from '@eslint/js';
import ts from 'typescript-eslint';
export default ts.config(js.configs.recommended, ...ts.configs.recommended, {
  files: ['src/**/*.{ts,tsx}'],
  languageOptions: {
    globals: {
      window: 'readonly',
      document: 'readonly',
      fetch: 'readonly',
      localStorage: 'readonly',
      console: 'readonly',
      URL: 'readonly',
      Blob: 'readonly',
      FormData: 'readonly',
      crypto: 'readonly',
      HTMLInputElement: 'readonly',
      HTMLElement: 'readonly',
      HTMLTextAreaElement: 'readonly',
      HTMLSelectElement: 'readonly',
      Image: 'readonly',
      ResizeObserver: 'readonly',
      setTimeout: 'readonly',
      clearTimeout: 'readonly',
      setInterval: 'readonly',
      clearInterval: 'readonly',
    },
  },
});
