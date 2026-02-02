<script lang="ts" setup>
import { computed, ref } from 'vue';

import { useVbenDrawer } from '@vben/common-ui';
import { $t } from '@vben/locales';

import { notification } from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { useServerStore } from '#/stores/server.state';

const serverStore = useServerStore();

const data = ref();

const getTitle = computed(() =>
  data.value?.create ? '新建托管者' : '编辑托管者',
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
      label: '托管者昵称',
      componentProps: {
        placeholder: '请输入托管者昵称',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入托管者昵称' }),
    },
    {
      component: 'Input',
      fieldName: 'ip',
      label: 'IP地址',
      componentProps: {
        placeholder: '请输入IP地址',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入IP地址' }),
    },
    {
      component: 'Input',
      fieldName: 'innerIp',
      label: '内网IP',
      componentProps: {
        placeholder: '请输入内网IP',
        allowClear: true,
      },
    },
    {
      component: 'InputNumber',
      fieldName: 'port',
      label: '端口',
      defaultValue: 8080,
      componentProps: {
        placeholder: '请输入端口',
        min: 1,
        max: 65535,
      },
      rules: z.number().min(1).max(65535),
    },
    {
      component: 'Input',
      fieldName: 'machineId',
      label: '机器ID',
      componentProps: {
        placeholder: '请输入机器ID',
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'vpcId',
      label: 'VPC ID',
      componentProps: {
        placeholder: '请输入VPC ID',
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'type',
      label: '服务器类型',
      defaultValue: 1,
      componentProps: {
        placeholder: '请选择服务器类型',
        options: [
          { label: '生产', value: 1 },
          { label: '测试', value: 2 },
        ],
      },
      rules: 'selectRequired',
    },
    {
      component: 'Select',
      fieldName: 'status',
      label: '服务器状态',
      defaultValue: 1,
      componentProps: {
        placeholder: '请选择服务器状态',
        options: [
          { label: '运行中', value: 1 },
          { label: '已停止', value: 2 },
          { label: '维护中', value: 3 },
        ],
      },
      rules: 'selectRequired',
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
        ? serverStore.createServer(values)
        : serverStore.updateServer(data.value.row.id, values));

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
    class="server-drawer"
  >
    <BaseForm />
  </Drawer>
</template>
