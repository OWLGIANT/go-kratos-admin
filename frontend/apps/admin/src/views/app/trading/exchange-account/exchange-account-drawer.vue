<script lang="ts" setup>
import type { VbenFormProps } from '@vben/common-ui';

import { computed, ref } from 'vue';

import { notification } from 'ant-design-vue';

import { $t } from '#/locales';
import { useExchangeAccountStore } from '#/stores/exchange-account.state';

interface Props {
  create: boolean;
  row?: any;
}

const props = defineProps<Props>();

const exchangeAccountStore = useExchangeAccountStore();

const loading = ref(false);

const title = computed(() => {
  return props.create ? '新建交易账号' : '编辑交易账号';
});

const formSchema: VbenFormProps['schema'] = [
  {
    component: 'Input',
    fieldName: 'nickname',
    label: '账号昵称',
    rules: 'required',
    componentProps: {
      placeholder: '请输入账号昵称',
    },
  },
  {
    component: 'Input',
    fieldName: 'exchangeName',
    label: '交易所名称',
    rules: 'required',
    componentProps: {
      placeholder: '请输入交易所名称',
    },
  },
  {
    component: 'Input',
    fieldName: 'originAccount',
    label: '原始账号',
    rules: 'required',
    componentProps: {
      placeholder: '请输入原始账号',
    },
  },
  {
    component: 'Input',
    fieldName: 'apiKey',
    label: 'API Key',
    rules: 'required',
    componentProps: {
      placeholder: '请输入API Key',
    },
  },
  {
    component: 'InputPassword',
    fieldName: 'secretKey',
    label: 'Secret Key',
    rules: 'required',
    componentProps: {
      placeholder: '请输入Secret Key',
    },
  },
  {
    component: 'InputPassword',
    fieldName: 'passKey',
    label: 'Pass Key',
    componentProps: {
      placeholder: '请输入Pass Key（可选）',
    },
  },
  {
    component: 'Input',
    fieldName: 'brokerId',
    label: '经纪商ID',
    componentProps: {
      placeholder: '请输入经纪商ID（可选）',
    },
  },
  {
    component: 'Select',
    fieldName: 'accountType',
    label: '账号类型',
    rules: 'required',
    defaultValue: 1,
    componentProps: {
      placeholder: '请选择账号类型',
      options: [
        { label: '自建', value: 1 },
        { label: '平台', value: 2 },
      ],
    },
  },
  {
    component: 'InputNumber',
    fieldName: 'specialReqLimit',
    label: '特殊限频',
    defaultValue: 0,
    componentProps: {
      placeholder: '请输入特殊限频',
      min: 0,
      style: { width: '100%' },
    },
  },
  {
    component: 'Input',
    fieldName: 'serverIps',
    label: '绑定托管者IP',
    componentProps: {
      placeholder: '多个IP用逗号分隔',
    },
  },
  {
    component: 'Textarea',
    fieldName: 'remark',
    label: '备注',
    componentProps: {
      placeholder: '请输入备注',
      rows: 3,
    },
  },
];

async function handleSubmit(values: Record<string, any>) {
  loading.value = true;
  try {
    if (props.create) {
      await exchangeAccountStore.createExchangeAccount(values);
      notification.success({
        message: '创建成功',
        description: `账号 ${values.nickname} 已创建`,
      });
    } else {
      await exchangeAccountStore.updateExchangeAccount(props.row.id, values);
      notification.success({
        message: '更新成功',
        description: `账号 ${values.nickname} 已更新`,
      });
    }
    return true;
  } catch (error: any) {
    notification.error({
      message: props.create ? '创建失败' : '更新失败',
      description: error.message || '操作失败',
    });
    return false;
  } finally {
    loading.value = false;
  }
}
</script>

<template>
  <VbenDrawer
    :loading="loading"
    :title="title"
    class="exchange-account-drawer"
    @submit="handleSubmit"
  >
    <template #default>
      <VbenForm
        :schema="formSchema"
        :values="create ? {} : row"
      />
    </template>
  </VbenDrawer>
</template>
