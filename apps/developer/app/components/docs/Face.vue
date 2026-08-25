<script setup lang="ts">
import { DOCS_FACE_META } from '~/constants/docs'
import { DEV_GRANTABLE_SCOPES } from '~/constants/dev'

const route = useRoute()
const { findFace, faceOperationCount } = useDocs()

const face = computed(() => findFace(route.params.face as string))

if (!face.value) {
  throw createError({ statusCode: 404, statusMessage: '未找到这个 API', fatal: true })
}

const current = computed(() => face.value!)
const meta = computed(() => DOCS_FACE_META[current.value.key])

const grantScope = computed(() => {
  const scopes = current.value.groups.flatMap((g) =>
    g.operations.map((o) => o.scope)
  )
  return scopes.find((s) =>
    (DEV_GRANTABLE_SCOPES as readonly string[]).includes(s)
  )
})

useSeoMeta({
  title: () => `${current.value.name} · API 文档`,
  description: () =>
    `${current.value.name} 的公开端点目录，共 ${faceOperationCount(current.value)} 个。`
})
</script>

<template>
  <div class="space-y-8">
    <nav class="flex items-center gap-1.5 text-sm text-default-400">
      <NuxtLink to="/docs" class="transition-colors hover:text-foreground">
        文档
      </NuxtLink>
      <KunIcon name="lucide:chevron-right" class="size-3.5" />
      <span class="text-default-500">{{ current.label }}</span>
    </nav>

    <header>
      <div class="flex flex-wrap items-center gap-2">
        <h1 class="text-2xl font-bold tracking-tight text-foreground">
          {{ current.name }}
        </h1>
        <span
          v-if="meta.badge"
          class="rounded-full bg-warning-50 px-2 py-0.5 text-xs font-medium text-warning-600"
        >
          {{ meta.badge }}
        </span>
      </div>
      <p class="mt-2 max-w-2xl text-sm leading-relaxed text-default-500">
        {{ meta.tagline }}
      </p>
      <div class="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
        <span class="text-default-500">
          {{ faceOperationCount(current) }} 个公开端点
        </span>
        <code class="font-mono text-xs text-default-400">
          {{ current.baseUrl }}{{ current.prefix }}
        </code>
        <code class="font-mono text-xs text-default-400">
          {{ current.auth.display }}
        </code>
      </div>
      <ul
        v-if="current.notes?.length"
        class="mt-4 space-y-2 rounded-xl border border-default-200 bg-content1 p-4"
      >
        <li
          v-for="(note, i) in current.notes"
          :key="i"
          class="text-sm leading-relaxed text-default-500"
        >
          {{ note }}
        </li>
      </ul>
      <p v-if="grantScope" class="mt-3 text-sm text-default-500">
        <NuxtLink to="/dashboard" class="text-primary hover:underline">
          去控制台申请 {{ grantScope }}
        </NuxtLink>
        —— 铸密钥的对话框里就能提交，也能看到自己这份申请是待审、已批准还是被拒（含理由）。
      </p>
    </header>

    <section v-for="group in current.groups" :key="group.key" class="space-y-3">
      <h2
        v-if="current.groups.length > 1"
        class="text-sm font-semibold text-foreground"
      >
        {{ group.label }}
      </h2>
      <ul class="divide-y divide-default-100 overflow-hidden rounded-xl border border-default-200">
        <li v-for="op in group.operations" :key="op.id">
          <NuxtLink
            :to="`/docs/${current.key}/${op.id}`"
            class="flex items-start gap-3 px-4 py-3 transition-colors hover:bg-default-50"
          >
            <DocsMethodBadge :method="op.method" size="md" />
            <div class="min-w-0 flex-1">
              <div class="flex flex-wrap items-center gap-2">
                <code class="font-mono text-sm text-foreground">{{ op.path }}</code>
                <span
                  v-if="op.auth?.kind === 'none'"
                  class="rounded bg-success-50 px-1.5 py-0.5 text-xs font-medium text-success-600"
                >
                  无需凭据
                </span>
              </div>
              <p class="mt-0.5 text-sm leading-relaxed text-default-500">
                {{ op.summary }}
              </p>
            </div>
            <KunIcon
              name="lucide:chevron-right"
              class="mt-1 size-4 shrink-0 text-default-300"
            />
          </NuxtLink>
        </li>
      </ul>
    </section>
  </div>
</template>
