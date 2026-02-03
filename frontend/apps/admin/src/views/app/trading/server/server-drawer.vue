<script lang="ts" setup>
import { computed, ref } from 'vue';

import { useVbenDrawer } from '@vben/common-ui';

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
      label: '昵称',
      componentProps: {
        placeholder: '请输入托管者昵称',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入托管者昵称' }),
    },
    {
      component: 'Input',
      fieldName: 'ip',
      label: '外网IP',
      componentProps: {
        placeholder: '请输入外网IP',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入外网IP' }),
    },
    {
      component: 'Input',
      fieldName: 'innerIp',
      label: '内网IP',
      componentProps: {
        placeholder: '请输入内网IP',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入内网IP' }),
    },
    {
      component: 'Input',
      fieldName: 'port',
      label: '端口',
      componentProps: {
        placeholder: '请输入端口',
        allowClear: true,
      },
      rules: z.string().min(1, { message: '请输入端口' }),
    },
    {
      component: 'Input',
      fieldName: 'machineId',
      label: '机器ID',
      componentProps: {
        placeholder: '请输入机器ID（可选）',
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'vpcId',
      label: 'VPC ID',
      componentProps: {
        placeholder: '请输入VPC ID（可选）',
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'instanceId',
      label: '实例ID',
      componentProps: {
        placeholder: '请输入实例ID（可选）',
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'type',
      label: '类型',
      defaultValue: 'SERVER_TYPE_SELF_BUILT',
      componentProps: {
        placeholder: '请选择类型',
        options: [
          { label: '自建', value: 'SERVER_TYPE_SELF_BUILT' },
          { label: '平台', value: 'SERVER_TYPE_PLATFORM' },
        ],
      },
      rules: 'selectRequired',
    },
    {
      component: 'Textarea',
      fieldName: 'remark',
      label: '备注',
      componentProps: {
        placeholder: '请输入备注（可选）',
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
