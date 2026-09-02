<script setup lang="ts">
import {
  CATALOG_FILTER_ALL,
  CATALOG_ENTITY_TYPES,
  CATALOG_ENTITY_TYPE,
  CATALOG_SOURCE_LABELS,
  CANDIDATE_STATUS,
  CANDIDATE_STATUS_LABELS,
  CANDIDATE_STATUS_COLORS,
  CANDIDATE_REASON_LABELS,
  WORK_STATUS,
  WORK_STATUS_LABELS
} from '~/constants/catalog'
import type {
  CatalogCandidateItem,
  CatalogCandidatePage,
  CatalogDecideCandidateData,
  CatalogDetachNameData
} from '~~/shared/types/catalog'

const isPersonLink = (c: CatalogCandidateItem) =>
  c.entity_type === CATALOG_ENTITY_TYPE.creditName

const isDecidable = (c: CatalogCandidateItem) =>
  (
    [
      CANDIDATE_STATUS.pending,
      CANDIDATE_STATUS.deferred,
      CANDIDATE_STATUS.needsManual
    ] as number[]
  ).includes(c.status)

const quarantinedIds = (c: CatalogCandidateItem) =>
  [
    { id: c.a_id, s: c.a },
    { id: c.b_id, s: c.b }
  ]
    .filter(({ s }) => s.work_status === WORK_STATUS.QUARANTINE)
    .map(({ id }) => id)

const quarantinedSide = (c: CatalogCandidateItem) => {
  if (c.a.work_status === WORK_STATUS.QUARANTINE) return 'a' as const
  if (c.b.work_status === WORK_STATUS.QUARANTINE) return 'b' as const
  return null
}

const api = useApi('catalog')

const page = ref(1)
const limit = 50
const status = ref(CANDIDATE_STATUS.pending as number)
const entityType = ref(CATALOG_FILTER_ALL)
const reason = ref(CATALOG_FILTER_ALL)
watch([status, entityType, reason], () => {
  page.value = 1
})

const { data, status: fetchStatus, refresh, error } =
  await useApiFetch<CatalogCandidatePage>(
    '/admin/catalog/candidates',
    {
      query: computed(() => ({
        page: page.value,
        limit,
        status: status.value,
        entity_type: entityType.value,
        reason: reason.value
      }))
    },
    'catalog'
  )
const items = computed(() => data.value?.items ?? [])
const total = computed(() => data.value?.total ?? 0)
const totalPages = computed(() => Math.max(1, Math.ceil(total.value / limit)))
const isLoading = computed(() => fetchStatus.value === 'pending')

const statusTabs = computed(() => [
  { id: CATALOG_FILTER_ALL, label: '全部' },
  ...Object.entries(CANDIDATE_STATUS_LABELS).map(([id, label]) => ({
    id: Number(id),
    label
  }))
])

const entityTypeOptions = computed(() => [
  { value: CATALOG_FILTER_ALL, label: '全部类型' },
  ...Object.entries(CATALOG_ENTITY_TYPES).map(([id, label]) => ({
    value: Number(id),
    label
  }))
])
const reasonFilterOptions = computed(() => [
  { value: CATALOG_FILTER_ALL, label: '全部来由' },
  ...Object.entries(CANDIDATE_REASON_LABELS).map(([id, label]) => ({
    value: Number(id),
    label
  }))
])

const expandedKey = ref('')
const direction = ref<'ab' | 'ba' | ''>('')
const note = ref('')
const deciding = ref(false)

const keyOf = (c: CatalogCandidateItem) =>
  `${c.entity_type}:${c.a_id}:${c.b_id}`

const toggleExpand = (c: CatalogCandidateItem) => {
  const key = keyOf(c)
  expandedKey.value = expandedKey.value === key ? '' : key
  const side = quarantinedSide(c)
  direction.value = side === 'a' ? 'ab' : side === 'b' ? 'ba' : ''
  note.value = ''
}

const decide = async (
  c: CatalogCandidateItem,
  action: 'accept' | 'reject' | 'defer'
) => {
  if (action === 'accept' && !isPersonLink(c) && !direction.value) {
    useKunMessage('请先显式选择合并方向', 'warn')
    return
  }
  deciding.value = true
  try {
    const body: Record<string, unknown> = {
      entity_type: c.entity_type,
      a_id: c.a_id,
      b_id: c.b_id,
      action,
      note: note.value || undefined
    }
    if (action === 'accept' && !isPersonLink(c)) {
      body.source_id = direction.value === 'ab' ? c.a_id : c.b_id
      body.target_id = direction.value === 'ab' ? c.b_id : c.a_id
    }
    const res = await api.post<CatalogDecideCandidateData>(
      '/admin/catalog/candidates/decide',
      body
    )
    if (res.code === 0) {
      if (action === 'accept' && isPersonLink(c)) {
        if (res.data?.needs_manual) {
          useKunMessage('双方已属不同 person，已标记待人工处理', 'warn')
        } else {
          useKunMessage(
            res.data?.person_created
              ? `已新建 person #${res.data?.person_id ?? ''}`
              : `已并入 person #${res.data?.person_id ?? ''}`,
            'success'
          )
        }
      } else if (action === 'accept') {
        useKunMessage(
          `已建合并提案 #${res.data?.proposal_id ?? ''}，请到提案桶审批`,
          'success'
        )
      } else if (res.data?.released?.length) {
        useKunMessage(
          res.data.released.map((id) => `已放行 #${id}`).join('、'),
          'success'
        )
      } else {
        useKunMessage(action === 'reject' ? '已拒绝（永久保留）' : '已搁置', 'success')
      }
      expandedKey.value = ''
      await refresh()
    } else {
      useKunMessage(res.message || '操作失败', 'error')
    }
  } finally {
    deciding.value = false
  }
}

const releaseWork = async (workId: number) => {
  deciding.value = true
  try {
    const res = await api.post<{ work_id: number; status: number }>(
      '/admin/catalog/works/release',
      { work_id: workId }
    )
    if (res.code === 0) {
      useKunMessage(`已放行 #${workId}`, 'success')
      expandedKey.value = ''
      await refresh()
    } else {
      useKunMessage(res.message || '操作失败', 'error')
    }
  } finally {
    deciding.value = false
  }
}

const detachLink = async (c: CatalogCandidateItem) => {
  deciding.value = true
  try {
    for (const id of [c.a_id, c.b_id]) {
      const res = await api.post<CatalogDetachNameData>(
        '/admin/catalog/names/detach',
        { credit_name_id: id }
      )
      if (res.code !== 0) {
        useKunMessage(res.message || '撤销失败', 'error')
        return
      }
    }
    useKunMessage('已撤销关联（双方名义已摘离）', 'success')
    expandedKey.value = ''
    await refresh()
  } finally {
    deciding.value = false
  }
}

const sourceLabel = (id: number | null | undefined) =>
  id == null ? '' : (CATALOG_SOURCE_LABELS[id] ?? `源${id}`)
</script>

<template>
  <div class="space-y-4">
    <h1 class="text-foreground text-2xl font-bold">目录人审队列</h1>
    <CatalogSubNav />

    <div class="flex flex-wrap items-center gap-2">
      <KunButton
        v-for="tab in statusTabs"
        :key="tab.id"
        :color="status === tab.id ? 'primary' : 'default'"
        :variant="status === tab.id ? 'solid' : 'flat'"
        size="sm"
        @click="status = tab.id"
      >
        {{ tab.label }}
      </KunButton>
      <KunSelect
        v-model="entityType"
        :options="entityTypeOptions"
        aria-label="实体类型过滤"
        class="w-40"
      />
      <KunSelect
        v-model="reason"
        :options="reasonFilterOptions"
        aria-label="候选来由过滤"
        class="w-40"
      />
    </div>
    <p class="text-default-500 text-sm">共 {{ total }} 条候选</p>

    <CommonFetchError v-if="error" @retry="refresh" />

    <div class="space-y-2">
      <KunCard
        v-for="c in items"
        :key="keyOf(c)"
        content-class="space-y-3 p-4"
      >
        <button
          class="flex w-full flex-wrap items-center gap-2 text-left"
          @click="toggleExpand(c)"
        >
          <KunChip color="info" variant="flat" size="xs">
            {{ CATALOG_ENTITY_TYPES[c.entity_type] ?? c.entity_type }}
          </KunChip>
          <span class="text-foreground font-medium">
            {{ c.a.display_name || `#${c.a_id}` }}
          </span>
          <KunIcon name="lucide:arrow-left-right" class="text-default-400 size-4" />
          <span class="text-foreground font-medium">
            {{ c.b.display_name || `#${c.b_id}` }}
          </span>
          <KunChip color="default" variant="flat" size="xs">
            {{ CANDIDATE_REASON_LABELS[c.reason] ?? c.reason }}
          </KunChip>
          <KunChip v-if="c.score !== null" color="default" variant="flat" size="xs">
            score {{ c.score?.toFixed(2) }}
          </KunChip>
          <KunChip
            :color="CANDIDATE_STATUS_COLORS[c.status] ?? 'default'"
            variant="flat"
            size="xs"
            class="ml-auto"
          >
            {{ CANDIDATE_STATUS_LABELS[c.status] ?? c.status }}
          </KunChip>
        </button>

        <div v-if="expandedKey === keyOf(c)" class="space-y-3">
          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div
              v-for="side in [
                { tag: 'A', s: c.a },
                { tag: 'B', s: c.b }
              ]"
              :key="side.tag"
              class="border-default-200 rounded-lg border p-3"
            >
              <p class="text-default-400 text-xs">{{ side.tag }} · #{{ side.s.id }}</p>
              <div class="flex flex-wrap items-center gap-1.5">
                <p
                  class="text-lg font-medium"
                  :class="
                    c.a.display_name !== c.b.display_name
                      ? 'text-warning-600'
                      : 'text-foreground'
                  "
                >
                  {{ side.s.display_name || '（无显示名）' }}
                </p>
                <KunChip
                  v-if="side.s.work_status === WORK_STATUS.QUARANTINE"
                  color="warning"
                  variant="flat"
                  size="sm"
                >
                  {{ WORK_STATUS_LABELS[WORK_STATUS.QUARANTINE] }}
                </KunChip>
              </div>
              <div
                v-if="isPersonLink(c)"
                class="mt-1 flex flex-wrap items-center gap-1.5"
              >
                <KunChip
                  v-if="side.s.source_id != null"
                  color="info"
                  variant="flat"
                  size="xs"
                >
                  {{ sourceLabel(side.s.source_id) }}
                </KunChip>
                <span class="text-default-400 text-xs">
                  {{ side.s.credit_count ?? 0 }} 条署名
                </span>
              </div>
            </div>
          </div>

          <template v-if="isPersonLink(c) && isDecidable(c)">
            <div class="flex flex-wrap gap-2">
              <KunButton
                color="success"
                :disabled="deciding"
                @click="decide(c, 'accept')"
              >
                <KunIcon
                  v-if="deciding"
                  name="lucide:loader-circle"
                  class="mr-1 size-4 animate-spin"
                />
                确认为同一人
              </KunButton>
              <KunButton
                color="danger"
                variant="flat"
                :disabled="deciding"
                @click="decide(c, 'reject')"
              >
                拒绝（永久保留）
              </KunButton>
              <KunButton
                color="default"
                variant="flat"
                :disabled="deciding"
                @click="decide(c, 'defer')"
              >
                搁置
              </KunButton>
            </div>
          </template>

          <template v-else-if="isDecidable(c)">
            <div class="flex flex-wrap items-center gap-2">
              <span class="text-default-500 text-sm">合并方向：</span>
              <KunButton
                :color="direction === 'ab' ? 'primary' : 'default'"
                :variant="direction === 'ab' ? 'solid' : 'flat'"
                size="sm"
                @click="direction = 'ab'"
              >
                A → B（保留 {{ c.b.display_name || `#${c.b_id}` }}）
              </KunButton>
              <KunButton
                :color="direction === 'ba' ? 'primary' : 'default'"
                :variant="direction === 'ba' ? 'solid' : 'flat'"
                size="sm"
                @click="direction = 'ba'"
              >
                B → A（保留 {{ c.a.display_name || `#${c.a_id}` }}）
              </KunButton>
            </div>
            <KunInput v-model="note" placeholder="备注（可选，随提案携带）" />
            <div class="flex flex-wrap gap-2">
              <KunButton
                color="success"
                :disabled="deciding || !direction"
                @click="decide(c, 'accept')"
              >
                <KunIcon
                  v-if="deciding"
                  name="lucide:loader-circle"
                  class="mr-1 size-4 animate-spin"
                />
                接受并建提案
              </KunButton>
              <KunButton
                color="danger"
                variant="flat"
                :disabled="deciding"
                @click="decide(c, 'reject')"
              >
                {{ quarantinedSide(c) ? '拒绝并放行' : '拒绝（永久保留）' }}
              </KunButton>
              <KunButton
                color="default"
                variant="flat"
                :disabled="deciding"
                @click="decide(c, 'defer')"
              >
                搁置
              </KunButton>
            </div>
          </template>

          <div v-else class="flex flex-wrap items-center gap-2">
            <p class="text-default-400 text-sm">
              该候选已决策（{{ CANDIDATE_STATUS_LABELS[c.status] }}
              <template v-if="c.decided_by !== null">
                · 操作者 {{ c.decided_by }}</template
              >）
            </p>
            <KunButton
              v-if="isPersonLink(c) && c.status === CANDIDATE_STATUS.accepted"
              color="default"
              variant="flat"
              size="sm"
              :disabled="deciding"
              @click="detachLink(c)"
            >
              撤销关联（摘离双方名义）
            </KunButton>
            <KunButton
              v-for="id in quarantinedIds(c)"
              :key="id"
              color="warning"
              variant="flat"
              :disabled="deciding"
              @click="releaseWork(id)"
            >
              放行 #{{ id }}
            </KunButton>
          </div>
        </div>
      </KunCard>

      <KunCard v-if="!items.length && !error" content-class="p-10">
        <p class="text-default-400 text-center">
          {{ isLoading ? '加载中…' : '没有匹配的候选' }}
        </p>
      </KunCard>
    </div>

    <KunPagination
      v-model:current-page="page"
      :total-page="totalPages"
      :is-loading="isLoading"
    />
  </div>
</template>
