<template>
  <div id="paymentManagePage">
    <div class="page-header">
      <div class="header-container">
        <div class="header-content">
          <h1 class="page-title">支付管理</h1>
          <p class="page-subtitle">全站支付记录（含 Mock VIP）</p>
        </div>
      </div>
    </div>

    <div class="container">
      <a-card :bordered="false" class="content-card">
        <div class="search-section">
          <a-form layout="inline" class="search-form" @finish="doSearch">
            <a-form-item label="状态">
              <a-select v-model:value="query.status" allow-clear placeholder="全部" style="width: 140px">
                <a-select-option value="PENDING">待支付</a-select-option>
                <a-select-option value="SUCCEEDED">成功</a-select-option>
                <a-select-option value="FAILED">失败</a-select-option>
                <a-select-option value="REFUNDED">已退款</a-select-option>
              </a-select>
            </a-form-item>
            <a-form-item label="用户 ID">
              <a-input-number v-model:value="query.userId" :min="1" placeholder="可选" style="width: 120px" />
            </a-form-item>
            <a-form-item label="产品">
              <a-input v-model:value="query.productType" placeholder="如 VIP_PERMANENT" style="width: 160px" allow-clear />
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
            <template v-if="column.key === 'amount'">
              {{ formatAmount(record) }}
            </template>
            <template v-else-if="column.key === 'status'">
              <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
            </template>
            <template v-else-if="column.key === 'session'">
              <span class="mono" :title="record.stripeSessionId">{{ shortId(record.stripeSessionId) }}</span>
            </template>
            <template v-else-if="column.key === 'createTime'">
              {{ formatTime(record.createTime) }}
            </template>
            <template v-else-if="column.key === 'updateTime'">
              {{ formatTime(record.updateTime) }}
            </template>
          </template>
        </a-table>
      </a-card>
    </div>
  </div>
</template>

<script lang="ts" setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import { adminListPaymentRecords } from '@/api/paymentController'

const loading = ref(false)
const data = ref<API.PaymentRecord[]>([])
const total = ref(0)
const query = reactive<{
  pageNum: number
  pageSize: number
  status?: string
  userId?: number
  productType?: string
}>({
  pageNum: 1,
  pageSize: 10,
})

const columns = [
  { title: 'ID', dataIndex: 'id', width: 80 },
  { title: '用户', dataIndex: 'userId', width: 90 },
  { title: '金额', key: 'amount', width: 120 },
  { title: '状态', key: 'status', width: 110 },
  { title: '产品', dataIndex: 'productType', width: 140 },
  { title: 'Session', key: 'session', width: 160 },
  { title: '描述', dataIndex: 'description', ellipsis: true },
  { title: '创建时间', key: 'createTime', width: 180 },
  { title: '更新时间', key: 'updateTime', width: 180 },
]

const pagination = computed(() => ({
  current: query.pageNum,
  pageSize: query.pageSize,
  total: total.value,
  showSizeChanger: true,
  showTotal: (n: number) => `共 ${n} 条`,
}))

const statusText = (s?: string) =>
  ({ PENDING: '待支付', SUCCEEDED: '成功', FAILED: '失败', REFUNDED: '已退款' } as Record<string, string>)[
    s || ''
  ] || s || '-'

const statusColor = (s?: string) =>
  ({ PENDING: 'default', SUCCEEDED: 'success', FAILED: 'error', REFUNDED: 'warning' } as Record<string, string>)[
    s || ''
  ] || 'default'

const formatAmount = (r: API.PaymentRecord) => {
  const cur = (r.currency || 'usd').toUpperCase()
  const n = r.amount ?? 0
  return `${n} ${cur}`
}

const formatTime = (v?: string) => (v ? dayjs(v).format('YYYY-MM-DD HH:mm:ss') : '-')
const shortId = (s?: string) => {
  if (!s) return '-'
  return s.length > 16 ? `${s.slice(0, 8)}…${s.slice(-4)}` : s
}

const fetchData = async () => {
  loading.value = true
  try {
    const res = await adminListPaymentRecords({
      pageNum: query.pageNum,
      pageSize: query.pageSize,
      status: query.status || undefined,
      userId: query.userId || undefined,
      productType: query.productType?.trim() || undefined,
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

onMounted(fetchData)
</script>

<style scoped lang="scss">
#paymentManagePage {
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
  .mono {
    font-family: ui-monospace, monospace;
    font-size: 12px;
  }
}
</style>
