<template>
  <div id="articleManagePage">
    <div class="page-header">
      <div class="header-container">
        <div class="header-content">
          <h1 class="page-title">文章管理</h1>
          <p class="page-subtitle">全站文章列表（管理员）</p>
        </div>
      </div>
    </div>

    <div class="container">
      <a-card :bordered="false" class="content-card">
        <div class="search-section">
          <a-form layout="inline" class="search-form" @finish="doSearch">
            <a-form-item label="状态">
              <a-select
                v-model:value="query.status"
                allow-clear
                placeholder="全部"
                style="width: 140px"
              >
                <a-select-option value="PENDING">等待中</a-select-option>
                <a-select-option value="PROCESSING">生成中</a-select-option>
                <a-select-option value="COMPLETED">已完成</a-select-option>
                <a-select-option value="FAILED">失败</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item>
              <a-button type="primary" html-type="submit">查询</a-button>
            </a-form-item>
          </a-form>
        </div>

        <a-table
          :columns="columns"
          :data-source="data"
          :loading="loading"
          :pagination="pagination"
          row-key="id"
          @change="onTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'title'">
              <div class="title-cell">
                <div class="main">{{ record.mainTitle || record.topic || '-' }}</div>
                <div class="sub">{{ record.subTitle || record.taskId }}</div>
              </div>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
            </template>
            <template v-else-if="column.key === 'phase'">
              <span class="mono">{{ record.phase || '-' }}</span>
            </template>
            <template v-else-if="column.key === 'createTime'">
              {{ formatTime(record.createTime) }}
            </template>
            <template v-else-if="column.key === 'action'">
              <a-space>
                <a-button type="link" size="small" @click="goDetail(record)">查看</a-button>
                <a-popconfirm
                  title="确定删除该文章？"
                  ok-text="删除"
                  cancel-text="取消"
                  @confirm="doDelete(record)"
                >
                  <a-button type="link" danger size="small">删除</a-button>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-card>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import { deleteArticle, listArticleAdmin } from '@/api/articleController'

const router = useRouter()
const loading = ref(false)
const data = ref<API.Article[]>([])
const total = ref(0)
const query = reactive<API.ArticleQueryRequest>({
  pageNum: 1,
  pageSize: 10,
  status: undefined,
})

const columns = [
  { title: 'ID', dataIndex: 'id', width: 80 },
  { title: '用户', dataIndex: 'userId', width: 90 },
  { title: '标题 / 选题', key: 'title' },
  { title: '状态', key: 'status', width: 110 },
  { title: '阶段', key: 'phase', width: 160 },
  { title: '创建时间', key: 'createTime', width: 180 },
  { title: '操作', key: 'action', width: 160 },
]

const pagination = computed(() => ({
  current: query.pageNum ?? 1,
  pageSize: query.pageSize ?? 10,
  total: total.value,
  showSizeChanger: true,
  showTotal: (n: number) => `共 ${n} 条`,
}))

const statusText = (s?: string) =>
  ({ PENDING: '等待中', PROCESSING: '生成中', COMPLETED: '已完成', FAILED: '失败' } as Record<string, string>)[
    s || ''
  ] || s || '-'

const statusColor = (s?: string) =>
  ({ PENDING: 'default', PROCESSING: 'processing', COMPLETED: 'success', FAILED: 'error' } as Record<
    string,
    string
  >)[s || ''] || 'default'

const formatTime = (v?: string) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-')

const fetchData = async () => {
  loading.value = true
  try {
    const res = await listArticleAdmin({
      pageNum: query.pageNum,
      pageSize: query.pageSize,
      status: query.status || undefined,
    })
    if (res.data.code === 0 && res.data.data) {
      data.value = res.data.data.records ?? []
      total.value = res.data.data.total ?? 0
    } else {
      message.error(res.data.message || '加载失败')
    }
  } catch (e) {
    message.error(e instanceof Error ? e.message : '加载失败')
  } finally {
    loading.value = false
  }
}

const doSearch = () => {
  query.pageNum = 1
  fetchData()
}

const onTableChange = (page: { current?: number; pageSize?: number }) => {
  query.pageNum = page.current ?? 1
  query.pageSize = page.pageSize ?? 10
  fetchData()
}

const goDetail = (row: API.Article) => {
  if (!row.taskId) return
  router.push(`/article/${row.taskId}`)
}

const doDelete = async (row: API.Article) => {
  if (!row.id) return
  try {
    const res = await deleteArticle({ id: row.id })
    if (res.data.code === 0) {
      message.success('已删除')
      fetchData()
    } else {
      message.error(res.data.message || '删除失败')
    }
  } catch (e) {
    message.error(e instanceof Error ? e.message : '删除失败')
  }
}

onMounted(fetchData)
</script>

<style scoped lang="scss">
#articleManagePage {
  background: var(--color-background-secondary);
  min-height: 100vh;
  padding-bottom: 60px;

  .page-header {
    background: var(--gradient-hero);
    padding: 32px 20px;
    margin-bottom: 24px;
  }
  .header-container {
    max-width: 1200px;
    margin: 0 auto;
  }
  .page-title {
    font-size: 28px;
    font-weight: 700;
    margin: 0 0 6px;
  }
  .page-subtitle {
    margin: 0;
    color: var(--color-text-secondary);
    font-size: 14px;
  }
  .container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 0 20px;
  }
  .content-card {
    border-radius: var(--radius-lg);
    border: 1px solid var(--color-border);
    :deep(.ant-card-body) {
      padding: 24px;
    }
  }
  .search-section {
    margin-bottom: 16px;
  }
  .title-cell .main {
    font-weight: 600;
  }
  .title-cell .sub {
    font-size: 12px;
    color: var(--color-text-secondary);
  }
  .mono {
    font-family: ui-monospace, monospace;
    font-size: 12px;
  }
}
</style>
