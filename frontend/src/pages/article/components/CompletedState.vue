<template>
  <div class="completed-state">
    <div class="success-header">
      <CheckCircleFilled class="success-icon" />
      <span>文章创作完成！</span>
    </div>

    <div class="preview-header">
      <h1 class="article-title">{{ article.mainTitle }}</h1>
      <p class="article-subtitle">{{ article.subTitle }}</p>
    </div>
    <div class="content-preview">
      <div ref="previewRef" v-html="markdownToHtml(article.fullContent || article.content || '')" class="markdown-body"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { CheckCircleFilled } from '@ant-design/icons-vue'
import { markdownToHtml, renderMermaid } from '@/utils/markdown'

const props = defineProps<{
  article: Partial<API.ArticleVO>
}>()

const previewRef = ref<HTMLElement | null>(null)

watch(
  () => props.article.fullContent || props.article.content,
  async () => {
    await nextTick()
    await renderMermaid(previewRef.value)
  },
  { immediate: true },
)
</script>

<style scoped lang="scss">
.completed-state {
  max-width: 100%;
}

.success-header {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 20px;
  background: var(--gradient-primary);
  border-radius: var(--radius-full);
  margin-bottom: 24px;
  color: white;
  font-size: 14px;
  font-weight: 600;

  .success-icon {
    font-size: 16px;
  }
}

/* 标题区域 */
.preview-header {
  text-align: center;
  margin-bottom: 24px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--color-border-light);
}

.article-title {
  font-size: 28px;
  font-weight: 700;
  margin: 0 0 8px;
  color: var(--color-text);
  line-height: 1.4;
}

.article-subtitle {
  font-size: 16px;
  color: var(--color-text-secondary);
  margin: 0;
}

/* 正文预览 */
.content-preview {
  line-height: 1.8;
}

.markdown-body {
  line-height: 1.8;
  font-size: 15px;
  color: var(--color-text);

  :deep(h2) {
    font-size: 20px;
    font-weight: 600;
    margin: 24px 0 14px;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--color-border);
    color: var(--color-text);
  }

  :deep(p) {
    margin-bottom: 14px;
    text-indent: 2em;
  }

  :deep(img) {
    display: block;
    max-width: 100%;
    margin: 20px auto;
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md);
  }
}
</style>
