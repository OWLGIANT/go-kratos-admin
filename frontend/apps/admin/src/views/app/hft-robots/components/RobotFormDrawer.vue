<template>
  <a-drawer
    v-model:open="visible"
    :title="isEdit ? '编辑机器人' : '新建机器人'"
    width="650"
    :body-style="{ paddingBottom: '80px' }"
    @close="handleClose"
  >
    <a-form
      ref="formRef"
      :model="formData"
      :rules="rules"
      layout="vertical"
      @finish="handleSubmit"
    >
      <a-form-item label="机器人名称" name="nickname">
        <a-input v-model:value="formData.nickname" placeholder="请输入机器人名称" />
      </a-form-item>

      <a-form-item label="账号" name="account">
        <a-select v-model:value="formData.account" placeholder="请选择账号">
          <a-select-option value="Account_A">Account_A</a-select-option>
          <a-select-option value="Account_B">Account_B</a-select-option>
          <a-select-option value="Account_C">Account_C</a-select-option>
          <a-select-option value="Account_D">Account_D</a-select-option>
        </a-select>
      </a-form-item>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item label="交易所" name="exchange">
            <a-select v-model:value="formData.exchange" placeholder="请选择交易所">
              <a-select-option
                v-for="opt in exchangeOptions"
                :key="opt.value"
                :value="opt.value"
              >
                {{ opt.label }}
              </a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item label="交易对" name="pair">
            <a-select
              v-model:value="formData.pair"
              placeholder="请选择交易对"
              show-search
            >
              <a-select-option value="BTC/USDT">BTC/USDT</a-select-option>
              <a-select-option value="ETH/USDT">ETH/USDT</a-select-option>
              <a-select-option value="SOL/USDT">SOL/USDT</a-select-option>
              <a-select-option value="BNB/USDT">BNB/USDT</a-select-option>
              <a-select-option value="DOGE/USDT">DOGE/USDT</a-select-option>
              <a-select-option value="ADA/USDT">ADA/USDT</a-select-option>
              <a-select-option value="XRP/USDT">XRP/USDT</a-select-option>
              <a-select-option value="MATIC/USDT">MATIC/USDT</a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
      </a-row>

      <a-row :gutter="16">
        <a-col :span="12">
          <a-form-item label="策略" name="strategy">
            <a-select
              v-model:value="formData.strategy"
              placeholder="请选择策略"
              @change="handleStrategyChange"
            >
              <a-select-option
                v-for="strategy in strategyOptions"
                :key="strategy.id"
                :value="strategy.id"
              >
                {{ strategy.name }}
              </a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
        <a-col :span="12">
          <a-form-item label="策略版本" name="strategyVersion">
            <a-select
              v-model:value="formData.strategyVersion"
              placeholder="请选择版本"
              :disabled="!formData.strategy"
            >
              <a-select-option
                v-for="version in availableVersions"
                :key="version"
                :value="version"
              >
                {{ version }}
              </a-select-option>
            </a-select>
          </a-form-item>
        </a-col>
      </a-row>

      <!-- Dynamic strategy parameters -->
      <template v-if="currentStrategy">
        <a-divider orientation="left">策略参数</a-divider>
        <a-alert
          :message="currentStrategy.description"
          type="info"
          show-icon
          style="margin-bottom: 16px"
        />

        <a-form-item
          v-for="param in currentStrategy.params"
          :key="param.name"
          :label="param.description"
          :name="['strategyParams', param.name]"
        >
          <template #label>
            <span>{{ param.description }}</span>
            <a-tooltip :title="param.tips">
              <QuestionCircleOutlined style="margin-left: 4px; color: #999" />
            </a-tooltip>
          </template>

          <!-- Number input (int or float64) -->
          <a-input-number
            v-if="param.type === 'int' || param.type === 'float64'"
            v-model:value="formData.strategyParams[param.name]"
            :precision="param.type === 'int' ? 0 : undefined"
            :min="param.min"
            :max="param.max"
            :step="param.type === 'int' ? 1 : 0.001"
            style="width: 100%"
          />

          <!-- String input -->
          <a-input
            v-else-if="param.type === 'string'"
            v-model:value="formData.strategyParams[param.name]"
          />

          <!-- Boolean checkbox -->
          <a-checkbox
            v-else-if="param.type === 'bool'"
            v-model:checked="formData.strategyParams[param.name]"
          >
            启用
          </a-checkbox>

          <!-- Select dropdown -->
          <a-select
            v-else-if="param.type === 'selected'"
            v-model:value="formData.strategyParams[param.name]"
            style="width: 100%"
          >
            <a-select-option v-for="opt in param.options" :key="opt" :value="opt">
              {{ opt }}
            </a-select-option>
          </a-select>
        </a-form-item>
      </template>

      <a-form-item label="备注" name="remark">
        <a-textarea
          v-model:value="formData.remark"
          :rows="3"
          placeholder="请输入备注信息"
        />
      </a-form-item>
    </a-form>

    <template #footer>
      <div style="text-align: right">
        <a-space>
          <a-button @click="handleClose">取消</a-button>
          <a-button type="primary" :loading="loading" @click="handleSubmit">
            {{ isEdit ? '更新' : '创建' }}
          </a-button>
        </a-space>
      </div>
    </template>
  </a-drawer>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { QuestionCircleOutlined } from '@ant-design/icons-vue';
import type { FormInstance } from 'ant-design-vue';
import type { Robot, RobotFormData } from '../types';
import { EXCHANGE_OPTIONS, MOCK_STRATEGIES, getStrategyById, getStrategyDefaultParams } from '../config';
import { useRobotStore } from '#/stores';

interface Props {
  visible: boolean;
  robot?: Robot | null;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:visible': [value: boolean];
  success: [];
}>();

const robotStore = useRobotStore();
const formRef = ref<FormInstance>();
const loading = ref(false);

const visible = computed({
  get: () => props.visible,
  set: (value) => emit('update:visible', value),
});

const isEdit = computed(() => !!props.robot);

const exchangeOptions = EXCHANGE_OPTIONS;
const strategyOptions = MOCK_STRATEGIES;

const formData = ref<RobotFormData>({
  nickname: '',
  account: '',
  exchange: 'okx_spot',
  pair: '',
  strategy: '',
  strategyVersion: '',
  strategyParams: {},
  remark: '',
});

const rules = {
  nickname: [{ required: true, message: '请输入机器人名称', trigger: 'blur' }],
  account: [{ required: true, message: '请选择账号', trigger: 'change' }],
  exchange: [{ required: true, message: '请选择交易所', trigger: 'change' }],
  pair: [{ required: true, message: '请选择交易对', trigger: 'change' }],
  strategy: [{ required: true, message: '请选择策略', trigger: 'change' }],
  strategyVersion: [{ required: true, message: '请选择策略版本', trigger: 'change' }],
};

const currentStrategy = computed(() => {
  if (!formData.value.strategy) return null;
  return getStrategyById(formData.value.strategy);
});

const availableVersions = computed(() => {
  return currentStrategy.value?.versions || [];
});

// Watch for robot prop changes to populate form
watch(
  () => props.robot,
  (newRobot) => {
    if (newRobot) {
      formData.value = {
        id: newRobot.id,
        nickname: newRobot.nickname,
        account: newRobot.account,
        exchange: newRobot.exchange,
        pair: newRobot.pair,
        strategy: newRobot.strategy,
        strategyVersion: newRobot.strategyVersion,
        strategyParams: { ...newRobot.strategyParams },
        remark: newRobot.remark,
      };
    } else {
      resetForm();
    }
  },
  { immediate: true },
);

const handleStrategyChange = (strategyId: string) => {
  // Reset strategy version and params when strategy changes
  formData.value.strategyVersion = '';
  formData.value.strategyParams = getStrategyDefaultParams(strategyId);

  // Auto-select latest version
  const strategy = getStrategyById(strategyId);
  if (strategy && strategy.versions.length > 0) {
    formData.value.strategyVersion = strategy.versions[strategy.versions.length - 1];
  }
};

const handleSubmit = async () => {
  try {
    await formRef.value?.validate();
    loading.value = true;

    let result;
    if (isEdit.value && formData.value.id) {
      result = await robotStore.updateRobot(formData.value.id, formData.value);
    } else {
      result = await robotStore.createRobot(formData.value);
    }

    if (result.success) {
      emit('success');
      handleClose();
    }
  } catch (error) {
    console.error('Form validation failed:', error);
  } finally {
    loading.value = false;
  }
};

const handleClose = () => {
  visible.value = false;
  resetForm();
};

const resetForm = () => {
  formData.value = {
    nickname: '',
    account: '',
    exchange: 'okx_spot',
    pair: '',
    strategy: '',
    strategyVersion: '',
    strategyParams: {},
    remark: '',
  };
  formRef.value?.resetFields();
};
</script>

<style scoped>
:deep(.ant-form-item-label > label) {
  font-weight: 500;
}

:deep(.ant-divider-inner-text) {
  font-weight: 600;
  font-size: 14px;
}
</style>
