<script lang="ts" setup>
import { computed, ref } from 'vue';

import { useVbenDrawer } from '@vben/common-ui';

import { notification } from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { useRobotStore, robotStatusList } from '#/stores/robot.state';
import { useServerStore } from '#/stores/server.state';
import { useExchangeAccountStore } from '#/stores/exchange-account.state';

const robotStore = useRobotStore();
const serverStore = useServerStore();
const exchangeAccountStore = useExchangeAccountStore();

const data = ref();

const getTitle = computed(() =>
  data.value?.create ? '新建Robot' : '编辑Robot',
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
      fieldName: 'robotId',
      label: '机器人ID',
      componentProps: {
        placeholder: '请输入机器人ID',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入机器人ID' }),
    },
    {
      component: 'Input',
      fieldName: 'nickname',
      label: '昵称',
      componentProps: {
        placeholder: '请输入昵称',
        allowClear: true,
      },
    },
    {
      component: 'ApiSelect',
      fieldName: 'serverId',
      label: '服务器',
      componentProps: {
        placeholder: '请选择服务器',
        allowClear: true,
        showSearch: true,
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        afterFetch: (data: any[]) => {
          return data.map((item: any) => ({
            label: `${item.nickname} (${item.ip})`,
            value: item.id,
          }));
        },
        api: async () => {
          const result = await serverStore.listServer();
          return result.items || [];
        },
      },
      rules: z.number().min(1, { message: '请选择服务器' }),
    },
    {
      component: 'ApiSelect',
      fieldName: 'exchangeAccountId',
      label: '交易账号',
      componentProps: {
        placeholder: '请选择交易账号',
        allowClear: true,
        showSearch: true,
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        afterFetch: (data: any[]) => {
          return data.map((item: any) => ({
            label: `${item.nickname} (${item.exchangeName})`,
            value: item.id,
          }));
        },
        api: async () => {
          const result = await exchangeAccountStore.listAccounts();
          return result.items || [];
        },
      },
      rules: z.number().min(1, { message: '请选择交易账号' }),
    },
    {
      component: 'Input',
      fieldName: 'exchange',
      label: '交易所',
      componentProps: {
        placeholder: '请输入交易所',
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'version',
      label: '版本',
      componentProps: {
        placeholder: '请输入版本',
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'status',
      label: '状态',
      defaultValue: 'disconnected',
      componentProps: {
        placeholder: '请选择状态',
        options: robotStatusList.value,
      },
    },
    {
      component: 'InputNumber',
      fieldName: 'initBalance',
      label: '初始资金',
      componentProps: {
        placeholder: '请输入初始资金',
        min: 0,
        precision: 2,
        style: { width: '100%' },
      },
    },
    {
      component: 'InputNumber',
      fieldName: 'balance',
      label: '余额',
      componentProps: {
        placeholder: '请输入余额',
        min: 0,
        precision: 2,
        style: { width: '100%' },
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
        ? robotStore.createRobot(values)
        : robotStore.updateRobot(values));

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
    class="robot-drawer"
  >
    <BaseForm />
  </Drawer>
</template>
