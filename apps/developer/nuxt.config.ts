import tailwindcss from '@tailwindcss/vite'

export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',

  devtools: { enabled: false },

  extends: ['@kungal/ui-nuxt'],

  app: {
    head: {
      // Hex twins of --background in @kungal/ui-tokens (oklch 0.968 .004 286.33
      // / 0.1448 0 0). A UA reads theme-color before any CSS, so it cannot be a
      // token reference; drifting from the token shows up as a mobile browser
      // chrome that does not match the page it frames.
      meta: [
        {
          name: 'theme-color',
          media: '(prefers-color-scheme: light)',
          content: '#f4f4f7'
        },
        {
          name: 'theme-color',
          media: '(prefers-color-scheme: dark)',
          content: '#0a0a0a'
        }
      ],
      link: [
        { rel: 'icon', type: 'image/x-icon', href: '/favicon.ico' },
        {
          rel: 'icon',
          type: 'image/png',
          sizes: '32x32',
          href: '/favicon-32x32.png'
        },
        {
          rel: 'icon',
          type: 'image/png',
          sizes: '16x16',
          href: '/favicon-16x16.png'
        },
        {
          rel: 'apple-touch-icon',
          sizes: '180x180',
          href: '/apple-touch-icon.png'
        },
        { rel: 'manifest', href: '/site.webmanifest' },
        {
          rel: 'alternate',
          type: 'text/plain',
          href: '/llms.txt',
          title: 'llms.txt'
        },
        {
          rel: 'alternate',
          type: 'text/plain',
          href: '/llms-full.txt',
          title: 'llms-full.txt'
        }
      ]
    }
  },

  css: ['~/assets/css/main.css'],

  routeRules: {
    '/explore': { headers: { 'X-Robots-Tag': 'noindex, nofollow' } },
    '/explore/**': { headers: { 'X-Robots-Tag': 'noindex, nofollow' } },
    '/relay/**': { headers: { 'X-Robots-Tag': 'noindex, nofollow' } }
  },

  modules: [
    '@nuxt/eslint',
    '@nuxtjs/color-mode',
    '@pinia/nuxt',
    'pinia-plugin-persistedstate/nuxt'
  ],

  devServer: {
    host: '127.0.0.1',
    port: 9430
  },

  pinia: {
    storesDirs: ['./store/**']
  },

  piniaPluginPersistedstate: {
    cookieOptions: {
      maxAge: 60 * 60 * 24 * 7,
      sameSite: 'strict'
    }
  },

  colorMode: {
    preference: 'system',
    fallback: 'light',
    globalName: '__NEXTMOE_DEV_COLOR_MODE__',
    componentName: 'ColorScheme',
    classPrefix: 'kun-',
    classSuffix: '-mode',
    storageKey: 'nextmoe-dev-color-mode'
  },

  vite: {
    // @ts-expect-error ts-expect-error
    plugins: [tailwindcss()]
  },

  runtimeConfig: {
    oauthApiBase: process.env.NUXT_OAUTH_API_BASE || 'http://127.0.0.1:19277',

    oauthClientSecret: process.env.NUXT_OAUTH_CLIENT_SECRET || '',

    nextmoeApiBase:
      process.env.NUXT_NEXTMOE_API_BASE || 'https://api.nextmoe.dev',

    public: {
      oauthAuthorizeBase:
        process.env.NUXT_PUBLIC_OAUTH_AUTHORIZE_BASE ||
        'http://127.0.0.1:9277/api/v1',
      oauthWebBase:
        process.env.NUXT_PUBLIC_OAUTH_WEB_BASE || 'http://127.0.0.1:9420',
      oauthClientId: process.env.NUXT_PUBLIC_OAUTH_CLIENT_ID || '',
      oauthRedirectUri:
        process.env.NUXT_PUBLIC_OAUTH_REDIRECT_URI ||
        'http://127.0.0.1:9430/auth/callback'
    }
  }
})
