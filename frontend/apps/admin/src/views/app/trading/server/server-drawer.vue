<script lang="ts" setup>
import type { VbenFormProps } from '@vben/common-ui';

import { computed, ref } from 'vue';

import { notification } from 'ant-design-vue';

import { $t } from '#/locales';
import { useServerStore } from '#/stores/server.state';

interface Props {
  create: boolean;
  row?: any;
}

const props = defineProps<Props>();

const serverStore = useServerStore();

const loading = ref(false);

const title = computed(() => {
  return props.create ? '新建托管者' : '编辑托管者';
});

const formSchema: VbenFormProps['schema'] = [
  {
    component: 'Input',
    fieldName: 'nickname',
    label: '托管者昵称',
    rules: 'required',
    componentProps: {
      placeholder: '请输入托管者昵称',
    },
  },
  {
    component: 'Input',
    fieldName: 'ip',
    label: 'IP地址',
    rules: 'required',
    componentProps: {
      placeholder: '请输入IP地址',
    },
  },
  {
    component: 'Input',
    fieldName: 'innerIp',
    label: '内网IP',
    componentProps: {
      placeholder: '请输入内网IP',
    },
  },
  {
    component: 'InputNumber',
    fieldName: 'port',
    label: '端口',
    rules: 'required',
    defaultValue: 8080,
    componentProps: {
      placeholder: '请输入端口',
      min: 1,
      max: 65535,
      style: { width: '100%' },
    },
  },
  {
    component: 'Input',
    fieldName: 'machineId',
    label: '机器ID',
    componentProps: {
      placeholder: '请输入机器ID',
    },
  },
  {
    component: 'Input',
    fieldName: 'vpcId',
    label: 'VPC ID',
    componentProps: {
      placeholder: '请输入VPC ID',
    },
  },
  {
    component: 'Select',
    fieldName: 'type',
    label: '服务器类型',
    rules: 'required',
    defaultValue: 1,
    componentProps: {
      placeholder: '请选择服务器类型',
      options: [
        { label: '生产', value: 1 },
        { label: '测试', value: 2 },
      ],
    },
  },
  {
    component: 'Select',
    fieldName: 'status',
    label: '服务器状态',
    rules: 'required',
    defaultValue: 1,
    componentProps: {
      placeholder: '请选择服务器状态',
      options: [
        { label: '运行中', value: 1 },
        { label: '已停止', value: 2 },
        { label: '维护中', value: 3 },
      ],
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
      await serverStore.createServer(values);
      notification.success({
        message: '创建成功',
        description: `托管者 ${values.nickname} 已创建`,
      });
    } else {
      await serverStore.updateServer(props.row.id, values);
      notification.success({
        message: '更新成功',
        description: `托管者 ${values.nickname} 已更新`,
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
    class="server-drawer"
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
