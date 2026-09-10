<template>
  <div id="paymentRecordsPage">
    <div class="page-header">
      <div class="header-container">
        <div class="header-content">
          <h1 class="page-title">我的支付记录</h1>
          <p class="page-subtitle">查看会员购买与相关订单</p>
        </div>
        <a-button type="primary" class="vip-btn" @click="goVip">
          <template #icon><CrownOutlined /></template>
          {{ isVip ? '会员中心' : '升级 VIP' }}
        </a-button>
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
          :locale="{ emptyText: emptyText }"
          @change="onTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'amount'">
              <span class="amount">{{ formatAmount(record) }}</span>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-tag :color="statusColor(record.status)">{{ statusText(record.status) }}</a-tag>
            </template>
            <template v-else-if="column.key === 'product'">
              {{ productLabel(record.productType) }}
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
import { useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { CrownOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import { getPaymentRecords } from '@/api/paymentController'
import { useLoginUserStore } from '@/stores/loginUser'
import { isVip as checkIsVip } from '@/utils/permission'

const router = useRouter()
const loginUserStore = useLoginUserStore()
const isVip = computed(() => checkIsVip(loginUserStore.loginUser))

const loading = ref(false)
const data = ref<API.PaymentRecord[]>([])
const total = ref(0)
const query = reactive<{
  pageNum: number
  pageSize: number
  status?: string
}>({
  pageNum: 1,
  pageSize: 10,
})

const columns = [
  { title: '订单 ID', dataIndex: 'id', width: 90 },
  { title: '产品', key: 'product', width: 140 },
  { title: '金额', key: 'amount', width: 120 },
  { title: '状态', key: 'status', width: 110 },
  { title: '会话号', key: 'session', width: 160 },
  { title: '说明', dataIndex: 'description', ellipsis: true },
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

const emptyText = computed(() =>
  loginUserStore.loginUser.id ? '暂无支付记录' : '请先登录',
)

const statusText = (s?: string) =>
  ({ PENDING: '待支付', SUCCEEDED: '成功', FAILED: '失败', REFUNDED: '已退款' } as Record<string, string>)[
    s || ''
  ] || s || '-'

const statusColor = (s?: string) =>
  ({ PENDING: 'default', SUCCEEDED: 'success', FAILED: 'error', REFUNDED: 'warning' } as Record<string, string>)[
    s || ''
  ] || 'default'

const productLabel = (t?: string) => {
  if (t === 'VIP_PERMANENT') return '永久 VIP'
  return t || '-'
}

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
  if (!loginUserStore.loginUser.id) {
    message.warning('请先登录')
    router.push('/user/login?redirect=' + encodeURIComponent('/payment/records'))
    return
  }
  loading.value = true
  try {
    const res = await getPaymentRecords({
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

const goVip = () => router.push('/vip')

onMounted(fetchData)
</script>

<style scoped lang="scss">
#paymentRecordsPage {
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
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    flex-wrap: wrap;
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

  .vip-btn {
    border-radius: var(--radius-md);
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

  .amount {
    font-weight: 600;
  }

  .mono {
    font-family: ui-monospace, monospace;
    font-size: 12px;
  }
}
</style>
