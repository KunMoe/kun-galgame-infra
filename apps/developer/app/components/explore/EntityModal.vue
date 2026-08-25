<script setup lang="ts">
interface Target {
  kind: 'characters' | 'credit-names' | 'companies' | 'works'
  id: string
}
interface Trait {
  id: number
  name: string
  group?: string
  sexual?: boolean
  spoiler?: number
  lie?: boolean
}
interface AttachRow {
  work?: { id: number; display_name?: string; content_rating?: string }
  voices?: { id: number; name: string }[]
  roles?: { role_key: string; role_name: string; character?: string | null }[]
  kind?: string
}

const props = defineProps<{ apiKey: string; nsfw: boolean }>()
const target = defineModel<Target | null>({ required: true })

const open = computed({
  get: () => target.value !== null,
  set: (v: boolean) => {
    if (!v) target.value = null
  }
})

const KIND_LABEL: Record<Target['kind'], string> = {
  characters: '角色',
  'credit-names': '名义',
  companies: '厂牌 / 社团',
  works: '作品'
}
const KIND_QUERY: Record<Target['kind'], Record<string, string>> = {
  characters: { view: 'full' },
  'credit-names': {},
  companies: {},
  works: { include: 'relations,credits' }
}

const loading = ref(false)
const error = ref('')
const data = ref<Record<string, unknown> | null>(null)
const showRaw = ref(false)

watch(target, async (t) => {
  data.value = null
  error.value = ''
  showRaw.value = false
  if (!t) return
  loading.value = true
  try {
    const qs = new URLSearchParams({
      ...KIND_QUERY[t.kind],
      ...(props.nsfw && { nsfw: 'true' })
    }).toString()
    const resp = await $fetch<Record<string, unknown>>(
      `/relay/v2/catalog/${t.kind}/${t.id}?${qs}`,
      { headers: { Authorization: `Bearer ${props.apiKey.trim()}` } }
    )
    data.value = resp ?? null
  } catch (e) {
    const err = e as { data?: { detail?: string; title?: string; message?: string }; statusCode?: number }
    error.value =
      err.data?.detail ??
      err.data?.title ??
      err.data?.message ??
      `请求失败（${err.statusCode ?? '网络错误'}）`
  } finally {
    loading.value = false
  }
})

const image = computed(() => {
  const v = data.value?.image
  return typeof v === 'string' && v ? v : null
})
const siblings = computed(
  () =>
    (data.value?.siblings as { id: number; name?: unknown }[] | undefined) ?? []
)
const siblingName = (sb: { id: number; name?: unknown }) =>
  entityName(sb as unknown as Record<string, unknown>) ?? `#${sb.id}`
const openSibling = (id: number) => {
  if (target.value) target.value = { kind: 'names', id }
}

const title = computed(() =>
  data.value ? (entityName(data.value) ?? `#${target.value?.id}`) : ''
)
const isR18 = computed(
  () => (data.value as { content_rating?: string } | null)?.content_rating === 'r18'
)
const refs = computed(
  () =>
    ((data.value?.refs as { source: string; external_id: string }[]) ?? []).slice(
      0,
      10
    )
)

const traitGroups = computed(() => {
  const list = (data.value?.traits as Trait[] | undefined) ?? []
  const m = new Map<string, Trait[]>()
  for (const t of list) {
    const g = t.group ?? '其他'
    const arr = m.get(g)
    if (arr) arr.push(t)
    else m.set(g, [t])
  }
  return [...m.entries()]
})

const intros = computed(
  () =>
    (data.value?.intros as { lang?: string; source?: string; intro: string }[]) ??
    []
)
const links = computed(
  () => (data.value?.links as { source?: string; url: string }[]) ?? []
)

const ROW_CAP = 15
const rows = computed<AttachRow[]>(() => {
  const d = data.value
  if (!d) return []
  const list = (d.works ?? d.credits) as AttachRow[] | undefined
  return Array.isArray(list) ? list.slice(0, ROW_CAP) : []
})
const rowsRest = computed(() => {
  const d = data.value
  if (!d) return 0
  const list = (d.works ?? d.credits) as unknown[] | undefined
  const over = Array.isArray(list) ? Math.max(0, list.length - ROW_CAP) : 0
  return over
})
const hasMorePages = computed(
  () => (data.value as { next_offset?: number } | null)?.next_offset != null
)

const goWork = (id?: number) => {
  if (!id) return
  target.value = null
  navigateTo(`/explore/work/${id}${props.nsfw ? '?nsfw=1' : ''}`)
}
</script>

<template>
  <KunModal v-model="open" size="lg">
    <div class="max-h-[70vh] space-y-4 overflow-y-auto pr-1">
      <p v-if="loading" class="py-8 text-center text-sm text-default-400">
        拉取实体档案…
      </p>
      <div v-else-if="error" class="rounded-lg bg-danger-50 p-3 text-sm text-danger">
        {{ error }}
      </div>

      <template v-else-if="data">
        <div class="flex flex-wrap items-center gap-2">
          <h2 class="text-lg font-semibold text-foreground">{{ title }}</h2>
          <KunChip color="default" variant="flat" size="xs">
            {{ KIND_LABEL[target?.kind ?? 'works'] }} #{{ target?.id }}
          </KunChip>
          <KunChip v-if="isR18" color="danger" variant="flat" size="xs">
            R18
          </KunChip>
        </div>

        <div
          v-if="image"
          class="w-28 overflow-hidden rounded-lg border border-default-200 bg-default-100"
        >
          <KunImageNative
            :src="image"
            :alt="title"
            loading="lazy"
            class-name="aspect-[3/4] w-full object-cover"
          />
        </div>

        <div v-if="siblings.length" class="flex flex-wrap items-center gap-1.5">
          <span class="text-xs text-default-400">同人格名义</span>
          <button
            v-for="sb in siblings"
            :key="sb.id"
            type="button"
            @click="openSibling(sb.id)"
          >
            <KunChip color="secondary" variant="flat" size="xs">
              {{ siblingName(sb) }}
            </KunChip>
          </button>
        </div>

        <div v-if="refs.length" class="flex flex-wrap items-center gap-1.5">
          <span class="text-xs text-default-400">外部标识</span>
          <KunChip
            v-for="rf in refs"
            :key="`${rf.source}-${rf.external_id}`"
            color="default"
            variant="flat"
            size="xs"
          >
            {{ rf.source }}:{{ rf.external_id }}
          </KunChip>
        </div>

        <div v-if="traitGroups.length" class="space-y-2">
          <h3 class="text-sm font-semibold text-foreground">Traits</h3>
          <div v-for="[g, ts] in traitGroups" :key="g">
            <p class="text-xs text-default-400">{{ g }}</p>
            <div class="mt-1 flex flex-wrap gap-1">
              <KunChip
                v-for="t in ts"
                :key="t.id"
                :color="t.sexual ? 'danger' : (t.spoiler ?? 0) > 0 ? 'warning' : 'default'"
                variant="flat"
                size="xs"
              >
                {{ t.name }}<template v-if="(t.spoiler ?? 0) > 0"> · 剧透{{ t.spoiler }}</template><template v-if="t.lie">（伪）</template>
              </KunChip>
            </div>
          </div>
        </div>

        <div v-if="intros.length" class="space-y-2">
          <h3 class="text-sm font-semibold text-foreground">简介</h3>
          <div
            v-for="(it, i) in intros"
            :key="`in-${i}`"
            class="rounded-lg border border-default-200 p-3"
          >
            <div class="mb-1 flex gap-1.5">
              <KunChip v-if="it.lang" color="default" variant="flat" size="xs">
                {{ it.lang }}
              </KunChip>
              <KunChip v-if="it.source" color="default" variant="flat" size="xs">
                {{ it.source }}
              </KunChip>
            </div>
            <p class="whitespace-pre-line text-xs leading-relaxed text-default-500">
              {{ it.intro }}
            </p>
          </div>
        </div>

        <div v-if="links.length" class="flex flex-wrap items-center gap-1.5">
          <span class="text-xs text-default-400">链接</span>
          <a
            v-for="(lk, i) in links"
            :key="`lk-${i}`"
            :href="lk.url"
            target="_blank"
            rel="noopener"
          >
            <KunChip color="secondary" variant="flat" size="xs">
              {{ lk.source ?? lk.url }}
            </KunChip>
          </a>
        </div>

        <div v-if="rows.length" class="space-y-1.5">
          <h3 class="text-sm font-semibold text-foreground">
            关联作品（{{ rows.length }}<template v-if="rowsRest">+{{ rowsRest }}</template><template v-if="hasMorePages">，API 可分页取更多</template>）
          </h3>
          <button
            v-for="(r, i) in rows"
            :key="`row-${i}`"
            type="button"
            class="flex w-full flex-wrap items-center gap-2 rounded-lg border border-default-200 px-3 py-2 text-left transition-colors hover:border-primary"
            @click="goWork(r.work?.id)"
          >
            <span class="min-w-0 flex-1 truncate text-sm text-foreground">
              {{ r.work?.display_name ?? `#${r.work?.id}` }}
            </span>
            <KunChip
              v-if="r.work?.content_rating === 'r18'"
              color="danger"
              variant="flat"
              size="xs"
            >
              R18
            </KunChip>
            <KunChip v-if="r.kind" color="default" variant="flat" size="xs">
              {{ r.kind }}
            </KunChip>
            <span v-if="r.voices?.length" class="text-xs text-default-400">
              CV {{ r.voices.map((v) => v.name).join(' / ') }}
            </span>
            <span v-if="r.roles?.length" class="text-xs text-default-400">
              {{ r.roles.map((ro) => ro.role_name + (ro.character ? `（${ro.character}）` : '')).join(' · ') }}
            </span>
          </button>
        </div>

        <div>
          <KunButton
            variant="light"
            color="default"
            size="xs"
            @click="showRaw = !showRaw"
          >
            {{ showRaw ? '收起完整数据' : '完整数据' }}
          </KunButton>
          <div v-if="showRaw" class="mt-1 max-h-64 overflow-auto rounded bg-default-100 p-2">
            <ExploreJsonTree :data="data" />
          </div>
        </div>
      </template>
    </div>
  </KunModal>
</template>
