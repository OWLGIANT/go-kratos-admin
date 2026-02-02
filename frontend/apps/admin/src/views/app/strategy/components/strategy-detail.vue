<template>
  <a-modal
    v-model:open="visible"
    title="策略详情"
    :footer="null"
    width="600px"
    @cancel="handleClose"
  >
    <div class="strategy-detail">
      <a-row
        v-for="param in params"
        :key="param.id"
        class="param-row"
        :gutter="16"
      >
        <a-col :span="8" class="param-label">
          {{ param.description }}
        </a-col>
        <a-col :span="16" class="param-value">
          {{ param.defaultValue }}
        </a-col>
      </a-row>
    </div>
  </a-modal>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { StrategyParam } from '#/stores';

interface Props {
  open: boolean;
  params: StrategyParam[];
}

interface Emits {
  (e: 'update:open', value: boolean): void;
}

const props = defineProps<Props>();
const emit = defineEmits<Emits>();

const visible = computed({
  get: () => props.open,
  set: (value) => emit('update:open', value),
});

const handleClose = () => {
  emit('update:open', false);
};
</script>

<style scoped lang="scss">
.strategy-detail {
  .param-row {
    padding: 12px 0;
    border-bottom: 1px solid #f0f0f0;

    &:last-child {
      border-bottom: none;
    }

    .param-label {
      font-weight: 500;
      color: #666;
    }

    .param-value {
      color: #333;
    }
  }
}
</style>
