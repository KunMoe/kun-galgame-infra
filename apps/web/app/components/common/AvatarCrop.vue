<script setup lang="ts">
const file = defineModel<Blob | null>('file', { required: true })

defineProps<{ disabled?: boolean }>()

const uploadKey = ref(0)

const onCropped = (blob: Blob) => {
  file.value = blob
}

const reset = () => {
  file.value = null
  uploadKey.value++
}

defineExpose({ reset })
</script>

<template>
  <div :class="disabled ? 'pointer-events-none opacity-50' : ''">
    <KunUpload
      :key="uploadKey"
      :size="512"
      :aspect="1"
      description="点击或拖拽选择图片，可裁剪为正方形"
      class-name="mx-auto w-48"
      @set-image="onCropped"
    />
  </div>
</template>
