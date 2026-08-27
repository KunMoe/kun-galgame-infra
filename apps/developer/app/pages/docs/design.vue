<script setup lang="ts">
definePageMeta({ layout: 'docs' })

useSeoMeta({
  title: 'API 设计原则',
  description: 'NextMoe Public API v2 的设计原则、客户端契约与稳定性承诺。'
})
</script>

<template>
  <div class="space-y-8">
    <header>
      <p class="text-sm font-medium tracking-wide text-primary">契约</p>
      <h1 class="mt-2 text-2xl font-bold text-foreground">API 设计原则</h1>
      <p class="mt-2 max-w-2xl text-sm text-default-500">
        v2 的形状来自
        <code class="font-mono text-xs">refs/api-v2</code>
        ，不是从 v1 翻过来的。这一页是对外发布的那几句——不写下来，加法演进就只是我们单方面的假设。
      </p>
    </header>

    <section class="rounded-xl border border-primary-200 bg-primary-50 p-4 text-sm text-primary-600">
      <strong>正式公开（2026-08-25）。</strong>
      /v2 对所有开发者开放：在门户自助铸造
      <code class="font-mono text-xs">nmk_</code>
      密钥即可调用，不需要申请。形状按 additive-only 演进——已发布的
      <code class="font-mono text-xs">v2-openapi.yaml</code>
      受 CI 的 oasdiff 破坏性变更门看守，删除与改名过不去；真要破坏只能升主版本。
    </section>

    <section class="space-y-3">
      <h2 class="text-lg font-semibold text-foreground">客户端契约（必须）</h2>
      <ul class="list-disc space-y-2 pl-5 text-sm text-default-500">
        <li>客户端必须忽略响应中未知的字段。</li>
        <li>客户端必须容忍开放词表中未见过的取值。</li>
        <li>客户端必须为未知的错误 <code class="font-mono text-xs">code</code> 准备一个按 HTTP <code class="font-mono text-xs">status</code> 的兜底分支。</li>
      </ul>
    </section>

    <section class="space-y-3">
      <h2 class="text-lg font-semibold text-foreground">几条承重原则</h2>
      <ul class="list-disc space-y-2 pl-5 text-sm text-default-500">
        <li>一个 URL 决定一个表示。没有通用信封，资源直接是 body。</li>
        <li>错误是 RFC 9457 <code class="font-mono text-xs">application/problem+json</code>。<code class="font-mono text-xs">type</code> 解析到本站 <NuxtLink to="/problems" class="text-primary hover:underline">/problems/…</NuxtLink>。</li>
        <li>面向机器的文本是英文。id 是十进制字符串。数组和 map 永不 <code class="font-mono text-xs">null</code>。</li>
        <li>默认瘦、按需胖：<code class="font-mono text-xs">view=basic|full</code> 与 <code class="font-mono text-xs">include=</code> / <code class="font-mono text-xs">fields=</code>。</li>
        <li>每个 id 都有地址。每个族都有批量读（<code class="font-mono text-xs">ids=</code> / <code class="font-mono text-xs">refs=</code>）。每个枚举都能在 <NuxtLink to="/docs/vocabularies" class="text-primary hover:underline">词表页</NuxtLink> 被发现。</li>
      </ul>
    </section>
  </div>
</template>
