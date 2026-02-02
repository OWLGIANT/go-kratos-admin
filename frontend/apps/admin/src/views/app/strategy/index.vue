<template>
  <div class="strategy-list-page">
    <a-page-header title="策略管理" />

    <div class="operation-bar">
      <a-space>
        <a-button type="primary" @click="handleAddStrategy">
          <template #icon>
            <FolderAddOutlined />
          </template>
          新建策略模板
        </a-button>
        <a-button type="primary" @click="handleSyncStrategy">
          同步策略
        </a-button>
      </a-space>
    </div>

    <div class="table-container">
      <a-table
        :columns="columns"
        :data-source="strategyStore.strategyList"
        :loading="strategyStore.loading"
        :pagination="false"
        row-key="id"
        bordered
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'strategyParams'">
            <a-button type="link" @click="handleViewDetail(record.strategyParams)">
              查看策略参数
            </a-button>
          </template>

          <template v-if="column.key === 'isUsingHub'">
            {{ record.isUsingHub ? '是' : '否' }}
          </template>

          <template v-if="column.key === 'createTime'">
            {{ formatTime(record.createTime) }}
          </template>

          <template v-if="column.key === 'operation'">
            <a-space>
              <a-button type="link" @click="handleEdit(record.id)">
                编辑
              </a-button>
              <a-popconfirm
                title="温馨提醒"
                :description="`确定要删除【${record.strategyName}】这个策略吗？`"
                @confirm="handleDelete(record.id)"
              >
                <a-button type="link" danger>
                  删除
                </a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </div>

    <strategy-detail
      v-model:open="detailModalVisible"
      :params="selectedParams"
    />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { FolderAddOutlined } from '@ant-design/icons-vue';
import { message } from 'ant-design-vue';
import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc';
import { useStrategyStore, type StrategyParam } from '#/stores';
import StrategyDetail from './components/strategy-detail.vue';

dayjs.extend(utc);

const strategyStore = useStrategyStore();
const detailModalVisible = ref(false);
const selectedParams = ref<StrategyParam[]>([]);

const columns = [
  {
    title: 'ID',
    dataIndex: 'id',
    key: 'id',
    align: 'center',
    width: 80,
  },
  {
    title: '策略名称',
    dataIndex: 'strategyName',
    key: 'strategyName',
    align: 'center',
  },
  {
    title: '策略别名',
    dataIndex: 'strategyNickname',
    key: 'strategyNickname',
    align: 'center',
  },
  {
    title: '策略详情',
    dataIndex: 'strategyParams',
    key: 'strategyParams',
    align: 'center',
  },
  {
    title: '是否集中进程管理',
    dataIndex: 'isUsingHub',
    key: 'isUsingHub',
    align: 'center',
  },
  {
    title: '创建时间',
    dataIndex: 'createTime',
    key: 'createTime',
    align: 'center',
    width: 180,
  },
  {
    title: '操作',
    key: 'operation',
    align: 'center',
    width: 150,
  },
];

const formatTime = (time: string) => {
  return dayjs.utc(time, 'YYYY-MM-DD HH:mm:ss').local().format('YYYY-MM-DD HH:mm:ss');
};

const handleAddStrategy = () => {
  message.info('新建策略功能开发中...');
};

const handleEdit = (id: number) => {
  message.info(`编辑策略 ID: ${id}`);
};

const handleDelete = async (id: number) => {
  await strategyStore.deleteStrategy(id);
};

const handleViewDetail = (params: StrategyParam[]) => {
  selectedParams.value = params;
  detailModalVisible.value = true;
};

const handleSyncStrategy = async () => {
  await strategyStore.syncStrategy();
};

onMounted(() => {
  strategyStore.getStrategyList();
});
</script>

<style scoped lang="scss">
.strategy-list-page {
  padding: 16px;

  .operation-bar {
    margin: 16px 0;
  }

  .table-container {
    background: #fff;
    padding: 16px;
    border-radius: 4px;
  }
}
</style>
