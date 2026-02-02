<template>
  <a-modal
    v-model:open="visible"
    title="批量修改参数"
    width="600px"
    @ok="handleOk"
    @cancel="handleCancel"
  >
    <a-alert
      :message="`已选择 ${robotIds.length} 个机器人`"
      type="info"
      show-icon
      style="margin-bottom: 16px"
    />

    <a-form ref="formRef" :model="formData" layout="vertical">
      <a-form-item label="选择要修改的参数">
        <a-select
          v-model:value="selectedParam"
          placeholder="请选择参数"
          style="width: 100%"
          @change="handleParamChange"
        >
          <a-select-option
            v-for="param in availableParams"
            :key="param.name"
            :value="param.name"
          >
            {{ param.description }}
          </a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item v-if="currentParam" :label="currentParam.description">
        <template #label>
          <span>{{ currentParam.description }}</span>
          <a-tooltip :title="currentParam.tips">
            <QuestionCircleOutlined style="margin-left: 4px; color: #999" />
          </a-tooltip>
        </template>

        <!-- Number input -->
        <a-input-number
          v-if="currentParam.type === 'int' || currentParam.type === 'float64'"
          v-model:value="paramValue"
          :precision="currentParam.type === 'int' ? 0 : undefined"
          :min="currentParam.min"
          :max="currentParam.max"
          :step="currentParam.type === 'int' ? 1 : 0.001"
          style="width: 100%"
        />

        <!-- String input -->
        <a-input v-else-if="currentParam.type === 'string'" v-model:value="paramValue" />

        <!-- Boolean checkbox -->
        <a-checkbox v-else-if="currentParam.type === 'bool'" v-model:checked="paramValue">
          启用
        </a-checkbox>

        <!-- Select dropdown -->
        <a-select
          v-else-if="currentParam.type === 'selected'"
          v-model:value="paramValue"
          style="width: 100%"
        >
          <a-select-option v-for="opt in currentParam.options" :key="opt" :value="opt">
            {{ opt }}
          </a-select-option>
        </a-select>
      </a-form-item>

      <a-alert
        message="注意"
        description="批量修改将覆盖所有选中机器人的该参数值，请谨慎操作"
        type="warning"
        show-icon
      />
    </a-form>
  </a-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { QuestionCircleOutlined } from '@ant-design/icons-vue';
import type { FormInstance } from 'ant-design-vue';
import type { StrategyParam } from '../types';
import { useRobotStore } from '#/stores';
import { getStrategyById } from '../config';

interface Props {
  visible: boolean;
  robotIds: number[];
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:visible': [value: boolean];
  success: [];
}>();

const robotStore = useRobotStore();
const formRef = ref<FormInstance>();

const visible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

const formData = ref({});
const selectedParam = ref<string>('');
const paramValue = ref<any>(null);

// Get common strategy from selected robots
const commonStrategy = computed(() => {
  if (props.robotIds.length === 0) return null;

  const robots = props.robotIds.map((id) => robotStore.getRobotById(id)).filter(Boolean);
  if (robots.length === 0) return null;

  const firstStrategy = robots[0]?.strategy;
  const allSameStrategy = robots.every((robot) => robot?.strategy === firstStrategy);

  return allSameStrategy ? firstStrategy : null;
});

// Get available parameters based on common strategy
const availableParams = computed<StrategyParam[]>(() => {
  if (!commonStrategy.value) return [];

  const strategy = getStrategyById(commonStrategy.value);
  return strategy?.params || [];
});

const currentParam = computed(() => {
  if (!selectedParam.value) return null;
  return availableParams.value.find((p) => p.name === selectedParam.value);
});

watch(
  () => props.visible,
  (newVal) => {
    if (newVal) {
      selectedParam.value = '';
      paramValue.value = null;
    }
  },
);

const handleParamChange = () => {
  if (currentParam.value) {
    paramValue.value = currentParam.value.default;
  }
};

const handleOk = async () => {
  if (!selectedParam.value || paramValue.value === null || paramValue.value === undefined) {
    return;
  }

  const params = {
    [selectedParam.value]: paramValue.value,
  };

  const result = await robotStore.batchModifyParams({
    robotIds: props.robotIds,
    params,
  });

  if (result.success) {
    emit('success');
    visible.value = false;
  }
};

const handleCancel = () => {
  visible.value = false;
};
</script>

<style scoped>
:deep(.ant-form-item-label > label) {
  font-weight: 500;
}
</style>
