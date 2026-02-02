<template>
  <div class="hft-robots-page">
    <a-page-header title="高频做市" />

    <!-- Filter Bar -->
    <FilterBar @filter="handleFilter" @reset="handleResetFilter" />

    <!-- Operation Bar -->
    <div class="operation-bar">
      <div class="operation-left">
        <a-space>
          <a-button type="primary" @click="handleAddRobot">
            <template #icon>
              <PlusOutlined />
            </template>
            新建机器人
          </a-button>
          <a-button @click="handleRefresh">
            <template #icon>
              <ReloadOutlined />
            </template>
            刷新
          </a-button>
          <a-button @click="showColumnSelector = true">
            <template #icon>
              <SettingOutlined />
            </template>
            列设置
          </a-button>

          <!-- Batch operations (shown when rows selected) -->
          <template v-if="hasSelected">
            <a-divider type="vertical" />
            <a-button @click="showBatchModify = true">
              <template #icon>
                <EditOutlined />
              </template>
              批量修改
            </a-button>
            <a-button danger @click="handleBatchDelete">
              <template #icon>
                <DeleteOutlined />
              </template>
              批量删除
            </a-button>
          </template>
        </a-space>
      </div>

      <div class="operation-right">
        <a-space>
          <ExportButton :selected-ids="selectedRowKeys" />
          <span v-if="hasSelected" class="selected-info">
            已选择 {{ selectedRowKeys.length }} 项
          </span>
        </a-space>
      </div>
    </div>

    <!-- Data Table -->
    <div class="table-container">
      <a-table
        :columns="visibleColumns"
        :data-source="filteredRobotList"
        :loading="robotStore.loading"
        :row-selection="rowSelection"
        :pagination="paginationConfig"
        :scroll="{ x: 'max-content' }"
        row-key="id"
        bordered
        size="middle"
      >
        <template #bodyCell="{ column, record }">
          <!-- Nickname with pin icon -->
          <template v-if="column.key === 'nickname'">
            <a-space>
              <PushpinFilled v-if="record.isPinned" style="color: #1890ff" />
              <span>{{ record.nickname }}</span>
            </a-space>
          </template>

          <!-- Equity curve chart -->
          <template v-if="column.key === 'equity'">
            <EquityChart :data="record.equityData" :current-balance="record.currentBalance" />
          </template>

          <!-- Yield with color -->
          <template v-if="column.key === 'yield'">
            <span :style="{ color: record.yield >= 0 ? '#52c41a' : '#ff4d4f', fontWeight: 500 }">
              {{ record.yield >= 0 ? '+' : '' }}{{ record.yield.toFixed(2) }}%
            </span>
          </template>

          <!-- Current balance -->
          <template v-if="column.key === 'currentBalance'">
            <span>{{ record.currentBalance.toFixed(2) }}</span>
          </template>

          <!-- Volume -->
          <template v-if="column.key === 'volume'">
            <span>{{ formatNumber(record.volume) }}</span>
          </template>

          <!-- Status with tag -->
          <template v-if="column.key === 'status'">
            <a-tag :color="getStatusColor(record.status)">
              {{ getStatusText(record.status) }}
            </a-tag>
          </template>

          <!-- Strategy params (expandable) -->
          <template v-if="column.key === 'strategyParams'">
            <ShowMore :content="JSON.stringify(record.strategyParams, null, 2)" :max-length="50" />
          </template>

          <!-- Exit message -->
          <template v-if="column.key === 'exitMsg'">
            <span v-if="record.exitMsg" style="color: #ff4d4f">{{ record.exitMsg }}</span>
            <span v-else style="color: #999">-</span>
          </template>

          <!-- Delays -->
          <template v-if="column.key === 'newDelay'">
            <span>{{ record.newDelay }}ms</span>
          </template>
          <template v-if="column.key === 'cancelDelay'">
            <span>{{ record.cancelDelay }}ms</span>
          </template>
          <template v-if="column.key === 'systemDelay'">
            <span>{{ record.systemDelay }}ms</span>
          </template>

          <!-- Fees -->
          <template v-if="column.key === 'makerFee'">
            <span>{{ (record.makerFee * 100).toFixed(4) }}%</span>
          </template>
          <template v-if="column.key === 'takerFee'">
            <span>{{ (record.takerFee * 100).toFixed(4) }}%</span>
          </template>

          <!-- Cash and Coin -->
          <template v-if="column.key === 'cash'">
            <span>{{ record.cash.toFixed(2) }}</span>
          </template>
          <template v-if="column.key === 'coin'">
            <span>{{ record.coin.toFixed(4) }}</span>
          </template>

          <!-- Operations -->
          <template v-if="column.key === 'operation'">
            <a-space :size="4" wrap>
              <a-button type="link" size="small" @click="handlePin(record)">
                {{ record.isPinned ? '取消置顶' : '置顶' }}
              </a-button>
              <a-button
                v-if="record.status === 'stopped' || record.status === 'error'"
                type="link"
                size="small"
                @click="handleStart(record.id)"
              >
                启动
              </a-button>
              <a-button
                v-if="record.status === 'running'"
                type="link"
                size="small"
                @click="handleStop(record.id)"
              >
                停止
              </a-button>
              <a-button type="link" size="small" @click="handleEdit(record)">
                编辑
              </a-button>
              <a-button type="link" size="small" @click="handleCopyParams(record)">
                复制参数
              </a-button>
              <a-button type="link" size="small" @click="handleViewHistory(record)">
                历史
              </a-button>
              <a-popconfirm
                title="确定要强制停止这个机器人吗？"
                @confirm="handleKill(record.id)"
              >
                <a-button type="link" size="small" danger>
                  强制停止
                </a-button>
              </a-popconfirm>
              <a-popconfirm
                title="确定要删除这个机器人吗？"
                @confirm="handleDelete(record.id)"
              >
                <a-button type="link" size="small" danger>
                  删除
                </a-button>
              </a-popconfirm>
            </a-space>
          </template>
        </template>
      </a-table>
    </div>

    <!-- Modals and Drawers -->
    <RobotFormDrawer
      v-model:visible="showRobotForm"
      :robot="selectedRobot"
      @success="handleRefresh"
    />
    <CopyParamsModal
      v-model:visible="showCopyParams"
      :source-robot="selectedRobot"
      @success="handleRefresh"
    />
    <BatchModifyModal
      v-model:visible="showBatchModify"
      :robot-ids="selectedRowKeys"
      @success="handleRefresh"
    />
    <ColumnSelector
      v-model:visible="showColumnSelector"
      v-model:columns="columnConfig"
    />
    <StatusHistoryModal
      v-model:visible="showStatusHistory"
      :robot-id="selectedRobot?.id"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import {
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
  EditOutlined,
  DeleteOutlined,
  PushpinFilled,
} from '@ant-design/icons-vue';
import { Modal } from 'ant-design-vue';
import type { TableColumnsType, TableProps } from 'ant-design-vue';
import { useRobotStore } from '#/stores';
import type { Robot, FilterConditions, ColumnConfig } from './types';
import { loadColumnConfig, getStatusConfig } from './config';
import FilterBar from './components/FilterBar.vue';
import EquityChart from './components/EquityChart.vue';
import ShowMore from './components/ShowMore.vue';
import RobotFormDrawer from './components/RobotFormDrawer.vue';
import CopyParamsModal from './components/CopyParamsModal.vue';
import BatchModifyModal from './components/BatchModifyModal.vue';
import ColumnSelector from './components/ColumnSelector.vue';
import StatusHistoryModal from './components/StatusHistoryModal.vue';
import ExportButton from './components/ExportButton.vue';

const robotStore = useRobotStore();

// Column configuration
const columnConfig = ref<ColumnConfig[]>(loadColumnConfig());

// Modal visibility states
const showRobotForm = ref(false);
const showCopyParams = ref(false);
const showBatchModify = ref(false);
const showColumnSelector = ref(false);
const showStatusHistory = ref(false);

// Selected robot for operations
const selectedRobot = ref<Robot | null>(null);

// Row selection
const selectedRowKeys = ref<number[]>([]);
const hasSelected = computed(() => selectedRowKeys.value.length > 0);

const rowSelection = computed<TableProps['rowSelection']>(() => ({
  selectedRowKeys: selectedRowKeys.value,
  onChange: (keys: any[]) => {
    selectedRowKeys.value = keys as number[];
  },
  getCheckboxProps: (record: Robot) => ({
    name: record.nickname,
  }),
}));

// Visible columns based on configuration
const visibleColumns = computed<TableColumnsType>(() => {
  return columnConfig.value
    .filter((col) => col.selected)
    .map((col) => ({
      title: col.label,
      dataIndex: col.key,
      key: col.key,
      width: col.width,
      fixed: col.fixed,
      align: ['yield', 'currentBalance', 'volume', 'cash', 'coin'].includes(col.key)
        ? 'right'
        : col.key === 'operation'
        ? 'center'
        : 'left',
    }));
});

// Filtered robot list
const filteredRobotList = computed(() => {
  return robotStore.robotList;
});

// Pagination config
const paginationConfig = computed(() => ({
  total: robotStore.total,
  pageSize: 20,
  showSizeChanger: true,
  showQuickJumper: true,
  showTotal: (total: number) => `共 ${total} 条记录`,
  pageSizeOptions: ['10', '20', '50', '100'],
}));

// Helper functions
const getStatusColor = (status: string): string => {
  const config = getStatusConfig(status);
  return config.color;
};

const getStatusText = (status: string): string => {
  const config = getStatusConfig(status);
  return config.label;
};

const formatNumber = (num: number): string => {
  if (num >= 1000000) {
    return (num / 1000000).toFixed(2) + 'M';
  } else if (num >= 1000) {
    return (num / 1000).toFixed(2) + 'K';
  }
  return num.toFixed(2);
};

// Event handlers
const handleFilter = (filters: FilterConditions) => {
  robotStore.applyFilters(filters);
  robotStore.getRobotList();
};

const handleResetFilter = () => {
  robotStore.clearFilters();
  robotStore.getRobotList();
};

const handleAddRobot = () => {
  selectedRobot.value = null;
  showRobotForm.value = true;
};

const handleEdit = (robot: Robot) => {
  selectedRobot.value = robot;
  showRobotForm.value = true;
};

const handleDelete = async (id: number) => {
  await robotStore.deleteRobot(id);
  selectedRowKeys.value = selectedRowKeys.value.filter((key) => key !== id);
};

const handleBatchDelete = () => {
  Modal.confirm({
    title: '批量删除确认',
    content: `确定要删除选中的 ${selectedRowKeys.value.length} 个机器人吗？此操作不可恢复。`,
    okText: '确定',
    okType: 'danger',
    cancelText: '取消',
    onOk: async () => {
      await robotStore.batchDeleteRobots(selectedRowKeys.value);
      selectedRowKeys.value = [];
    },
  });
};

const handleStart = async (id: number) => {
  await robotStore.startRobot(id);
};

const handleStop = async (id: number) => {
  await robotStore.stopRobot(id);
};

const handleKill = async (id: number) => {
  await robotStore.killRobot(id);
};

const handlePin = async (robot: Robot) => {
  await robotStore.togglePinRobot(robot.id);
  await robotStore.getRobotList();
};

const handleCopyParams = (robot: Robot) => {
  selectedRobot.value = robot;
  showCopyParams.value = true;
};

const handleViewHistory = (robot: Robot) => {
  selectedRobot.value = robot;
  showStatusHistory.value = true;
};

const handleRefresh = () => {
  robotStore.getRobotList();
};

// Initialize
onMounted(() => {
  robotStore.getRobotList();
});
</script>

<style scoped lang="scss">
.hft-robots-page {
  padding: 16px;

  .operation-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin: 16px 0;
    padding: 12px 16px;
    background: #fff;
    border-radius: 4px;

    .operation-left {
      flex: 1;
    }

    .operation-right {
      .selected-info {
        color: #1890ff;
        font-weight: 500;
      }
    }
  }

  .table-container {
    background: #fff;
    padding: 16px;
    border-radius: 4px;

    :deep(.ant-table) {
      font-size: 13px;
    }

    :deep(.ant-table-cell) {
      padding: 8px 12px;
    }

    :deep(.ant-btn-link) {
      padding: 0 4px;
      height: auto;
    }
  }
}
</style>
