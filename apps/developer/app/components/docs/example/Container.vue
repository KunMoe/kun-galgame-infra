<script setup lang="ts">
useSeoMeta({
  title: '实战示例',
  description:
    '用两个真实系列走通 NextMoe 公开 API v2 全链：标题搜索 → 详情按 include 取块 → 系列/厂牌深链 → 外部 id 反查。'
})

const step1 = `curl "https://api.nextmoe.dev/v2/catalog/search?object=work&q=SEQUEL&nsfw=true" \\
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"`

const step1Resp = `{
  "object": "list",
  "items": [
    {
      "object": "search_result",
      "target_object": "work",
      "id": "207379",
      "display_name": "この素晴らしい世界に祝福を！このゲーム性に嫉妬を！",
      "latin": "…",
      "localized": { "zh-Hans": { "value": "…", "is_machine": false } },
      "sources": ["dlsite", "bangumi"],
      "content_rating": "r18"
    }
    // …更多命中（SEQUEL 系列 5 员都在索引里）
  ],
  "total": 5
}`

const step2 = `curl "https://api.nextmoe.dev/v2/catalog/works/207379?nsfw=true\\
&include=series,companies,popularity,titles,relations,credits" \\
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"`

const step2Resp = `{
  "object": "work",
  "id": "207379",
  "medium": "galgame",
  "display_name": "…",
  "content_rating": "r18",
  "release_status": "released",
  "series":    [{ "object": "series",  "id": "320",   "display_name": "SEQUEL", "member_count": 5, "source": "dlsite" }],
  "companies": [{ "object": "company", "id": "11310", "display_name": "リーフジオメトリ",
                  "company_kind": "doujin_circle", "attribution_role": "circle" }],
  "popularity": [
    { "source": "bangumi", "metric": "bgm_wish",    "value": 33 },
    { "source": "bangumi", "metric": "bgm_collect", "value": 29 },
    { "source": "dlsite",  "metric": "downloads",   "value": 17710 },
    { "source": "dlsite",  "metric": "wishlist",    "value": 17114 }
  ],
  "titles": [{ "lang": "ja", "title": "…", "latin": "…", "title_kind": "official", "is_machine": false }],
  "relations": [ /* include=relations：关系边 + 对端 work（view=basic） */ ],
  "credits":   [ /* include=credits：按 role_key 分组；CV 那条带 character_id */ ]
  // 还有 refs / intros / tags / covers / screenshots / releases / ratings /
  //   playtimes / characters / engines / links / platforms —— v2 默认瘦身，
  //   每一块都要在 include= 里点名，写错 token 是 400 UNKNOWN_INCLUDE 而不是静默少块
}`

const step2b = `// いろセカ家族（w426 紅い瞳）的多源评分与时长（include=ratings,playtimes）：
"ratings": [
  { "source": "vndb",         "score": 7.87, "vote_count": 375 },
  { "source": "bangumi",      "score": 6.9,  "vote_count": 1065, "rank": 4184 },
  { "source": "erogamescape", "score": 78,   "vote_count": 446 }
],
"playtimes": [
  { "source": "vndb",         "minutes": 578, "vote_count": 38 },
  { "source": "erogamescape", "minutes": 600 }
]
// rank / vote_count 这类「没记录」的位置 v2 恒发 null 或 0，不会整键消失`

const step3 = `curl "https://api.nextmoe.dev/v2/catalog/companies/11310" \\
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"`

const step3Resp = `{
  "object": "company",
  "id": "11310",
  "display_name": "リーフジオメトリ",
  "latin": "…",
  "localized": {},
  "company_kind": "doujin_circle"
  // work_count：同一 nsfw 闸下可见的作品数，恒在
}`

const step3b = `curl "https://api.nextmoe.dev/v2/catalog/works?company_id=11310&nsfw=true" \\
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"`

const step4 = `curl "https://api.nextmoe.dev/v2/catalog/works?refs=vndb:v19658&nsfw=true" \\
  -H "Authorization: Bearer nmk_live_<YOUR_KEY>"`
</script>

<template>
  <div class="space-y-10">
    <div>
      <h1 class="text-2xl font-bold text-foreground">
        实战示例：两个真实系列走通全链
      </h1>
      <p class="mt-2 text-sm leading-relaxed text-default-500">
        以 <b>SEQUEL</b>（同人 · リーフジオメトリ）与
        <b>いろセカ</b>（商业 · FAVORITE）两个真实系列演示 搜索 → 详情 → 系列/厂牌深链 →
        外部 id 反查，全部走公开 API v2。两系列均为 R18 —— 正好演示调用方自控的
        <code class="font-mono text-xs">nsfw</code> 参数（缺省隐藏，显式
        <code class="font-mono text-xs">nsfw=true</code> 才可见；只认
        <code class="font-mono text-xs">true</code> /
        <code class="font-mono text-xs">false</code>，写别的是 400 而不是按默认值处理）。
      </p>
    </div>

    <section class="space-y-3">
      <h2 class="text-lg font-semibold text-foreground">
        1. 标题搜索（object=work）
      </h2>
      <pre class="overflow-x-auto rounded-xl border border-default-200 bg-content1 p-4 text-xs leading-relaxed"><code>{{ step1 }}</code></pre>
      <pre class="overflow-x-auto rounded-xl border border-default-200 bg-content1 p-4 text-xs leading-relaxed text-default-500"><code>{{ step1Resp }}</code></pre>
      <p class="text-sm text-default-500">
        没有信封：响应体<b>就是</b>那个集合。id 一律是字符串，翻页用
        <code class="font-mono text-xs">next_cursor</code>（末页直接不出现这个键，没有
        <code class="font-mono text-xs">has_more</code>）。
      </p>
    </section>

    <section class="space-y-3">
      <h2 class="text-lg font-semibold text-foreground">
        2. 详情：默认瘦，按 include 取块
      </h2>
      <pre class="overflow-x-auto rounded-xl border border-default-200 bg-content1 p-4 text-xs leading-relaxed"><code>{{ step2 }}</code></pre>
      <pre class="overflow-x-auto rounded-xl border border-default-200 bg-content1 p-4 text-xs leading-relaxed text-default-500"><code>{{ step2Resp }}</code></pre>
      <p class="text-sm text-default-500">
        多源数据在同一响应里并列出（按
        <code class="font-mono text-xs">source</code> 区分，刻度保持源原生、绝不归一）：
      </p>
      <pre class="overflow-x-auto rounded-xl border border-default-200 bg-content1 p-4 text-xs leading-relaxed text-default-500"><code>{{ step2b }}</code></pre>
    </section>

    <section class="space-y-3">
      <h2 class="text-lg font-semibold text-foreground">3. 厂牌 / 社团档案</h2>
      <pre class="overflow-x-auto rounded-xl border border-default-200 bg-content1 p-4 text-xs leading-relaxed"><code>{{ step3 }}</code></pre>
      <pre class="overflow-x-auto rounded-xl border border-default-200 bg-content1 p-4 text-xs leading-relaxed text-default-500"><code>{{ step3Resp }}</code></pre>
      <p class="text-sm text-default-500">
        名下作品不内嵌在这个实体里 —— 反查是作品集合上的一个过滤器，这样它自带分页与全套过滤：
      </p>
      <pre class="overflow-x-auto rounded-xl border border-default-200 bg-content1 p-4 text-xs leading-relaxed"><code>{{ step3b }}</code></pre>
    </section>

    <section class="space-y-3">
      <h2 class="text-lg font-semibold text-foreground">
        4. 外部 id 反查（手握 VNDB/Bangumi/DLsite/EG id 时的首选）
      </h2>
      <pre class="overflow-x-auto rounded-xl border border-default-200 bg-content1 p-4 text-xs leading-relaxed"><code>{{ step4 }}</code></pre>
      <p class="text-sm text-default-500">
        批量就把最多 100 个 <code class="font-mono text-xs">source:external_id</code>
        用逗号连起来一次问完 —— v2 没有 <code class="font-mono text-xs">/lookup</code>
        这样的动词路径，反查是集合上的
        <code class="font-mono text-xs">refs=</code> 参数。没锚到的会原样回在
        <code class="font-mono text-xs">missing[]</code> 里，而不是让整个请求 404。
        全部端点与参数见
        <NuxtLink to="/docs/v2" class="text-primary hover:underline">
          API v2 文档
        </NuxtLink>；机器可读 spec 在
        <code class="font-mono text-xs">/v2/catalog/openapi.json</code>（免 key）。
        想先不写代码试一试？用
        <NuxtLink to="/explore" class="text-primary hover:underline">数据浏览</NuxtLink>。
      </p>
    </section>
  </div>
</template>
