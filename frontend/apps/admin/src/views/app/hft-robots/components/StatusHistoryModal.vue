<template>
  <a-modal
    v-model:open="visible"
    title="状态历史"
    width="700px"
    :footer="null"
    @cancel="handleCancel"
  >
    <a-spin :spinning="loading">
      <a-timeline v-if="historyList.length > 0" mode="left">
        <a-timeline-item
          v-for="item in historyList"
          :key="item.id"
          :color="getStatusColor(item.toStatus)"
        >
          <template #dot>
            <ClockCircleOutlined v-if="item.toStatus === 'running'" style="font-size: 16px" />
            <CheckCircleOutlined
              v-else-if="item.toStatus === 'stopped'"
              style="font-size: 16px"
            />
            <CloseCircleOutlined
              v-else-if="item.toStatus === 'error'"
              style="font-size: 16px"
            />
          </template>

          <div class="history-item">
            <div class="history-header">
              <span class="history-time">{{ item.timestamp }}</span>
              <span class="history-operator">操作人: {{ item.operator }}</span>
            </div>
            <div class="history-content">
              <a-tag :color="getStatusColor(item.fromStatus)">
                {{ getStatusText(item.fromStatus) }}
              </a-tag>
              <ArrowRightOutlined style="margin: 0 8px" />
              <a-tag :color="getStatusColor(item.toStatus)">
                {{ getStatusText(item.toStatus) }}
              </a-tag>
            </div>
            <div v-if="item.message" class="history-message">
              {{ item.message }}
            </div>
          </div>
        </a-timeline-item>
      </a-timeline>

      <a-empty v-else description="暂无状态历史记录" />
    </a-spin>
  </a-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import {
  ClockCircleOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  ArrowRightOutlined,
} from '@ant-design/icons-vue';
import type { StatusHistoryEntry, RobotStatus } from '../types';
import { useRobotStore } from '#/stores';
import { getStatusConfig } from '../config';

interface Props {
  visible: boolean;
  robotId?: number;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:visible': [value: boolean];
}>();

const robotStore = useRobotStore();
const loading = ref(false);
const historyList = ref<StatusHistoryEntry[]>([]);

const visible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

watch(
  () => props.visible,
  async (newVal) => {
    if (newVal && props.robotId) {
      await loadHistory();
    }
  },
);

const loadHistory = async () => {
  if (!props.robotId) return;

  loading.value = true;
  try {
    historyList.value = await robotStore.getStatusHistory(props.robotId);
  } catch (error) {
    console.error('Failed to load status history:', error);
  } finally {
    loading.value = false;
  }
};

const getStatusColor = (status: RobotStatus): string => {
  const config = getStatusConfig(status);
  return config.color;
};

const getStatusText = (status: RobotStatus): string => {
  const config = getStatusConfig(status);
  return config.label;
};

const handleCancel = () => {
  visible.value = false;
};
</script>

<style scoped>
.history-item {
  padding: 8px 0;
}

.history-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.history-time {
  font-weight: 500;
  color: #262626;
}

.history-operator {
  font-size: 12px;
  color: #8c8c8c;
}

.history-content {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
}

.history-message {
  font-size: 13px;
  color: #595959;
  padding: 8px 12px;
  background: #f5f5f5;
  border-radius: 4px;
  margin-top: 8px;
}

:deep(.ant-timeline-item-content) {
  margin-left: 24px;
}
</style>
