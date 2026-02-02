<script lang="ts" setup>
import { computed, ref } from 'vue';

import { useVbenDrawer } from '@vben/common-ui';
import { $t } from '@vben/locales';

import { notification } from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { useExchangeAccountStore } from '#/stores/exchange-account.state';

const exchangeAccountStore = useExchangeAccountStore();

const data = ref();

const getTitle = computed(() =>
  data.value?.create ? '新建交易账号' : '编辑交易账号',
);

const [BaseForm, baseFormApi] = useVbenForm({
  showDefaultActions: false,
  commonConfig: {
    componentProps: {
      class: 'w-full',
    },
  },
  schema: [
    {
      component: 'Input',
      fieldName: 'nickname',
      label: '账号昵称',
      componentProps: {
        placeholder: '请输入账号昵称',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入账号昵称' }),
    },
    {
      component: 'Input',
      fieldName: 'exchangeName',
      label: '交易所名称',
      componentProps: {
        placeholder: '请输入交易所名称',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入交易所名称' }),
    },
    {
      component: 'Input',
      fieldName: 'originAccount',
      label: '原始账号',
      componentProps: {
        placeholder: '请输入原始账号',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入原始账号' }),
    },
    {
      component: 'Input',
      fieldName: 'apiKey',
      label: 'API Key',
      componentProps: {
        placeholder: '请输入API Key',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入API Key' }),
    },
    {
      component: 'InputPassword',
      fieldName: 'secretKey',
      label: 'Secret Key',
      componentProps: {
        placeholder: '请输入Secret Key',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入Secret Key' }),
    },
    {
      component: 'InputPassword',
      fieldName: 'passKey',
      label: 'Pass Key',
      componentProps: {
        placeholder: '请输入Pass Key（可选）',
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'brokerId',
      label: '经纪商ID',
      componentProps: {
        placeholder: '请输入经纪商ID（可选）',
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'accountType',
      label: '账号类型',
      defaultValue: 1,
      componentProps: {
        placeholder: '请选择账号类型',
        options: [
          { label: '自建', value: 1 },
          { label: '平台', value: 2 },
        ],
      },
      rules: 'selectRequired',
    },
    {
      component: 'InputNumber',
      fieldName: 'specialReqLimit',
      label: '特殊限频',
      defaultValue: 0,
      componentProps: {
        placeholder: '请输入特殊限频',
        min: 0,
      },
    },
    {
      component: 'Input',
      fieldName: 'serverIps',
      label: '绑定托管者IP',
      componentProps: {
        placeholder: '多个IP用逗号分隔',
        allowClear: true,
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
  ],
});

const [Drawer, drawerApi] = useVbenDrawer({
  onCancel() {
    drawerApi.close();
  },

  async onConfirm() {
    const validate = await baseFormApi.validate();
    if (!validate.valid) {
      return;
    }

    setLoading(true);

    const values = await baseFormApi.getValues();

    try {
      await (data.value?.create
        ? exchangeAccountStore.createExchangeAccount(values)
        : exchangeAccountStore.updateExchangeAccount(data.value.row.id, values));

      notification.success({
        message: data.value?.create ? '创建成功' : '更新成功',
      });

      drawerApi.close();
    } catch (error: any) {
      notification.error({
        message: data.value?.create ? '创建失败' : '更新失败',
        description: error.message || '操作失败',
      });
    } finally {
      setLoading(false);
    }
  },

  onOpenChange(isOpen: boolean) {
    if (isOpen) {
      data.value = drawerApi.getData();
      if (!data.value?.create && data.value?.row) {
        baseFormApi.setValues(data.value.row);
      } else {
        baseFormApi.resetForm();
      }
    }
  },
});

function setLoading(loading: boolean) {
  drawerApi.setState({ loading });
}

defineExpose({
  Drawer,
  drawerApi,
});
</script>

<template>
  <Drawer
    :title="getTitle"
    class="exchange-account-drawer"
  >
    <BaseForm />
  </Drawer>
</template>
