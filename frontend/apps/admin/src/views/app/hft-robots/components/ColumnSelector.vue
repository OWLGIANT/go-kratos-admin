<template>
  <a-modal
    v-model:open="visible"
    title="列设置"
    width="600px"
    @ok="handleOk"
    @cancel="handleCancel"
  >
    <div class="column-selector">
      <a-alert
        message="提示"
        description="选择要显示的列，拖动可调整顺序"
        type="info"
        show-icon
        style="margin-bottom: 16px"
      />

      <div class="column-list">
        <a-checkbox-group v-model:value="selectedKeys" style="width: 100%">
          <div
            v-for="column in editableColumns"
            :key="column.key"
            class="column-item"
          >
            <a-checkbox :value="column.key">
              {{ column.label }}
            </a-checkbox>
          </div>
        </a-checkbox-group>
      </div>

      <div class="actions">
        <a-space>
          <a-button size="small" @click="selectAll">全选</a-button>
          <a-button size="small" @click="deselectAll">全不选</a-button>
          <a-button size="small" @click="resetToDefault">恢复默认</a-button>
        </a-space>
      </div>
    </div>
  </a-modal>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue';
import type { ColumnConfig } from '../types';
import { DEFAULT_COLUMNS, saveColumnConfig } from '../config';

interface Props {
  visible: boolean;
  columns: ColumnConfig[];
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:visible': [value: boolean];
  'update:columns': [columns: ColumnConfig[]];
}>();

const visible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

const selectedKeys = ref<string[]>([]);

// Editable columns (exclude fixed columns like ID and operation)
const editableColumns = computed(() => {
  return props.columns.filter((col) => col.key !== 'id' && col.key !== 'operation');
});

// Initialize selected keys from props
watch(
  () => props.visible,
  (newVal) => {
    if (newVal) {
      selectedKeys.value = props.columns.filter((col) => col.selected).map((col) => col.key);
    }
  },
  { immediate: true },
);

const handleOk = () => {
  const updatedColumns = props.columns.map((col) => ({
    ...col,
    selected:
      col.key === 'id' || col.key === 'operation' || selectedKeys.value.includes(col.key),
  }));

  emit('update:columns', updatedColumns);
  saveColumnConfig(updatedColumns);
  visible.value = false;
};

const handleCancel = () => {
  visible.value = false;
};

const selectAll = () => {
  selectedKeys.value = editableColumns.value.map((col) => col.key);
};

const deselectAll = () => {
  selectedKeys.value = [];
};

const resetToDefault = () => {
  selectedKeys.value = DEFAULT_COLUMNS.filter((col) => col.selected).map((col) => col.key);
};
</script>

<style scoped>
.column-selector {
  max-height: 500px;
}

.column-list {
  max-height: 400px;
  overflow-y: auto;
  border: 1px solid #f0f0f0;
  border-radius: 4px;
  padding: 12px;
  margin-bottom: 16px;
}

.column-item {
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
}

.column-item:last-child {
  border-bottom: none;
}

.column-item :deep(.ant-checkbox-wrapper) {
  width: 100%;
}

.actions {
  display: flex;
  justify-content: flex-end;
}
</style>
