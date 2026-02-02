<template>
  <a-modal
    v-model:open="visible"
    title="复制参数"
    width="600px"
    @ok="handleOk"
    @cancel="handleCancel"
  >
    <a-alert
      message="提示"
      description="将源机器人的策略参数复制到目标机器人。只能复制到使用相同策略的机器人。"
      type="info"
      show-icon
      style="margin-bottom: 16px"
    />

    <a-form ref="formRef" :model="formData" layout="vertical">
      <a-form-item label="源机器人">
        <a-input :value="sourceRobotName" disabled />
      </a-form-item>

      <a-form-item label="策略">
        <a-input :value="sourceStrategyName" disabled />
      </a-form-item>

      <a-form-item label="当前参数">
        <div class="params-preview">
          <pre>{{ paramsPreview }}</pre>
        </div>
      </a-form-item>

      <a-form-item label="目标机器人" required>
        <a-select
          v-model:value="selectedTargetIds"
          mode="multiple"
          placeholder="请选择目标机器人"
          style="width: 100%"
          :filter-option="filterOption"
        >
          <a-select-option
            v-for="robot in compatibleRobots"
            :key="robot.id"
            :value="robot.id"
          >
            {{ robot.nickname }} ({{ robot.pair }})
          </a-select-option>
        </a-select>
      </a-form-item>

      <a-alert
        v-if="selectedTargetIds.length > 0"
        :message="`将复制参数到 ${selectedTargetIds.length} 个机器人`"
        type="warning"
        show-icon
      />
    </a-form>
  </a-modal>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import type { FormInstance } from 'ant-design-vue';
import type { Robot } from '../types';
import { useRobotStore } from '#/stores';
import { getStrategyById } from '../config';

interface Props {
  visible: boolean;
  sourceRobot: Robot | null;
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
const selectedTargetIds = ref<number[]>([]);

const sourceRobotName = computed(() => props.sourceRobot?.nickname || '');

const sourceStrategyName = computed(() => {
  if (!props.sourceRobot) return '';
  const strategy = getStrategyById(props.sourceRobot.strategy);
  return strategy?.name || props.sourceRobot.strategy;
});

const paramsPreview = computed(() => {
  if (!props.sourceRobot) return '';
  return JSON.stringify(props.sourceRobot.strategyParams, null, 2);
});

// Get robots with the same strategy (excluding source robot)
const compatibleRobots = computed(() => {
  if (!props.sourceRobot) return [];

  return robotStore.robotList.filter(
    (robot) =>
      robot.id !== props.sourceRobot?.id && robot.strategy === props.sourceRobot?.strategy,
  );
});

watch(
  () => props.visible,
  (newVal) => {
    if (newVal) {
      selectedTargetIds.value = [];
    }
  },
);

const filterOption = (input: string, option: any) => {
  return option.children[0].children.toLowerCase().includes(input.toLowerCase());
};

const handleOk = async () => {
  if (!props.sourceRobot || selectedTargetIds.value.length === 0) {
    return;
  }

  const result = await robotStore.copyParams({
    sourceRobotId: props.sourceRobot.id,
    targetRobotIds: selectedTargetIds.value,
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
.params-preview {
  background: #f5f5f5;
  border: 1px solid #d9d9d9;
  border-radius: 4px;
  padding: 12px;
  max-height: 200px;
  overflow-y: auto;
}

.params-preview pre {
  margin: 0;
  font-size: 12px;
  font-family: 'Courier New', monospace;
}

:deep(.ant-form-item-label > label) {
  font-weight: 500;
}
</style>
