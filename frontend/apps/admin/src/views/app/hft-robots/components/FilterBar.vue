<template>
  <div class="filter-bar">
    <a-form layout="inline" :model="filters">
      <a-form-item label="交易所">
        <a-select
          v-model:value="filters.exchange"
          style="width: 150px"
          placeholder="全部"
          allow-clear
        >
          <a-select-option value="">全部</a-select-option>
          <a-select-option
            v-for="opt in exchangeOptions"
            :key="opt.value"
            :value="opt.value"
          >
            {{ opt.label }}
          </a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item label="交易对">
        <a-select
          v-model:value="filters.pair"
          style="width: 150px"
          placeholder="全部"
          allow-clear
          show-search
        >
          <a-select-option value="">全部</a-select-option>
          <a-select-option v-for="pair in pairOptions" :key="pair" :value="pair">
            {{ pair }}
          </a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item label="状态">
        <a-select
          v-model:value="filters.status"
          style="width: 120px"
          placeholder="全部"
          allow-clear
        >
          <a-select-option value="">全部</a-select-option>
          <a-select-option
            v-for="opt in statusOptions"
            :key="opt.value"
            :value="opt.value"
          >
            {{ opt.label }}
          </a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item label="策略">
        <a-select
          v-model:value="filters.strategy"
          style="width: 150px"
          placeholder="全部"
          allow-clear
        >
          <a-select-option value="">全部</a-select-option>
          <a-select-option
            v-for="strategy in strategyOptions"
            :key="strategy.id"
            :value="strategy.id"
          >
            {{ strategy.name }}
          </a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item label="创建人">
        <a-select
          v-model:value="filters.creator"
          style="width: 120px"
          placeholder="全部"
          allow-clear
        >
          <a-select-option value="">全部</a-select-option>
          <a-select-option v-for="creator in creatorOptions" :key="creator" :value="creator">
            {{ creator }}
          </a-select-option>
        </a-select>
      </a-form-item>

      <a-form-item label="关键词">
        <a-input
          v-model:value="filters.keyword"
          placeholder="搜索名称/账号"
          style="width: 200px"
          allow-clear
          @press-enter="handleSearch"
        />
      </a-form-item>

      <a-form-item>
        <a-space>
          <a-button type="primary" @click="handleSearch">
            <template #icon>
              <SearchOutlined />
            </template>
            搜索
          </a-button>
          <a-button @click="handleReset">
            <template #icon>
              <ReloadOutlined />
            </template>
            重置
          </a-button>
        </a-space>
      </a-form-item>
    </a-form>
  </div>
</template>

<script setup lang="ts">
import { reactive, computed } from 'vue';
import { SearchOutlined, ReloadOutlined } from '@ant-design/icons-vue';
import type { FilterConditions } from '../types';
import { EXCHANGE_OPTIONS, STATUS_OPTIONS, MOCK_STRATEGIES } from '../config';
import { useRobotStore } from '#/stores';

const emit = defineEmits<{
  filter: [filters: FilterConditions];
  reset: [];
}>();

const robotStore = useRobotStore();

const filters = reactive<FilterConditions>({
  exchange: '',
  pair: '',
  status: '',
  strategy: '',
  creator: '',
  keyword: '',
});

const exchangeOptions = EXCHANGE_OPTIONS;
const statusOptions = STATUS_OPTIONS;
const strategyOptions = MOCK_STRATEGIES;

// Get unique pairs from robot list
const pairOptions = computed(() => {
  const pairs = new Set<string>();
  robotStore.robotList.forEach((robot) => {
    pairs.add(robot.pair);
  });
  return Array.from(pairs).sort();
});

// Get unique creators from robot list
const creatorOptions = computed(() => {
  const creators = new Set<string>();
  robotStore.robotList.forEach((robot) => {
    creators.add(robot.creator);
  });
  return Array.from(creators).sort();
});

const handleSearch = () => {
  const filterConditions: FilterConditions = {
    exchange: filters.exchange || undefined,
    pair: filters.pair || undefined,
    status: filters.status as any,
    strategy: filters.strategy || undefined,
    creator: filters.creator || undefined,
    keyword: filters.keyword || undefined,
  };
  emit('filter', filterConditions);
};

const handleReset = () => {
  filters.exchange = '';
  filters.pair = '';
  filters.status = '';
  filters.strategy = '';
  filters.creator = '';
  filters.keyword = '';
  emit('reset');
};
</script>

<style scoped>
.filter-bar {
  padding: 16px;
  background: #fafafa;
  border-radius: 4px;
  margin-bottom: 16px;
}

.filter-bar :deep(.ant-form-item) {
  margin-bottom: 8px;
}
</style>
