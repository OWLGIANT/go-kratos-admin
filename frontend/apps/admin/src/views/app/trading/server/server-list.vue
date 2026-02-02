<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';

import { h } from 'vue';

import { useVbenDrawer, type VbenFormProps } from '@vben/common-ui';
import { LucideFilePenLine, LucideTrash2, LucideRefreshCw, LucideFileText } from '@vben/icons';

import { notification } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { useServerStore } from '#/stores/server.state';

import ServerDrawer from './server-drawer.vue';

const serverStore = useServerStore();

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'nickname',
      label: '托管者昵称',
      componentProps: {
        placeholder: '请输入托管者昵称',
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'ip',
      label: 'IP地址',
      componentProps: {
        placeholder: '请输入IP地址',
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'type',
      label: '服务器类型',
      componentProps: {
        placeholder: '请选择服务器类型',
        allowClear: true,
        options: [
          { label: '生产', value: 1 },
          { label: '测试', value: 2 },
        ],
      },
    },
  ],
};

const gridOptions: VxeGridProps = {
  height: 'auto',
  stripe: true,
  toolbarConfig: {
    custom: false,
    export: true,
    refresh: true,
    zoom: false,
  },
  exportConfig: {},
  pagerConfig: {},
  rowConfig: {
    isHover: true,
  },

  proxyConfig: {
    ajax: {
      query: async ({ page }, formValues) => {
        const result = await serverStore.listServer(
          { page: page.currentPage, pageSize: page.pageSize },
          formValues,
        );
        return {
          page: {
            total: Number(result.total || 0),
          },
          result: result.items || [],
        };
      },
    },
  },

  columns: [
    { title: 'ID', field: 'id', width: 80 },
    { title: '托管者昵称', field: 'nickname', minWidth: 150 },
    { title: 'IP地址', field: 'ip', width: 150 },
    { title: '内网IP', field: 'innerIp', width: 150 },
    { title: '端口', field: 'port', width: 100 },
    { title: '机器ID', field: 'machineId', width: 120 },
    {
      title: '服务器类型',
      field: 'type',
      width: 100,
      slots: { default: 'type' },
    },
    {
      title: '状态',
      field: 'status',
      width: 100,
      slots: { default: 'status' },
    },
    { title: '备注', field: 'remark', minWidth: 150 },
    {
      title: '操作',
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 180,
    },
  ],
};

const gridEvents: VxeGridListeners = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

const [Drawer, drawerApi] = useVbenDrawer({
  connectedComponent: ServerDrawer,

  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      gridApi.reload();
    }
  },
});

function openDrawer(create: boolean, row?: any) {
  drawerApi.setData({
    create,
    row,
  });
  drawerApi.open();
}

function handleCreate() {
  openDrawer(true);
}

function handleEdit(row: any) {
  openDrawer(false, row);
}

async function handleDelete(row: any) {
  try {
    await serverStore.deleteServer(row.id);
    notification.success({
      message: '删除成功',
      description: `托管者 ${row.nickname} 已删除`,
    });
    gridApi.reload();
  } catch (error: any) {
    notification.error({
      message: '删除失败',
      description: error.message || '删除托管者时发生错误',
    });
  }
}

async function handleReboot(row: any) {
  try {
    await serverStore.rebootServer(row.id);
    notification.success({
      message: '重启成功',
      description: `托管者 ${row.nickname} 已发送重启命令`,
    });
  } catch (error: any) {
    notification.error({
      message: '重启失败',
      description: error.message || '重启托管者时发生错误',
    });
  }
}

async function handleViewLog(row: any) {
  try {
    const result = await serverStore.getServerLog(row.id, 100);
    notification.info({
      message: '服务器日志',
      description: result.log || '暂无日志',
      duration: 10,
    });
  } catch (error: any) {
    notification.error({
      message: '获取日志失败',
      description: error.message || '获取日志时发生错误',
    });
  }
}

function getServerTypeText(type: number) {
  return type === 1 ? '生产' : '测试';
}

function getServerStatusText(status: number) {
  const statusMap: Record<number, string> = {
    1: '运行中',
    2: '已停止',
    3: '维护中',
  };
  return statusMap[status] || '未知';
}

function getServerStatusColor(status: number) {
  const colorMap: Record<number, string> = {
    1: 'green',
    2: 'red',
    3: 'orange',
  };
  return colorMap[status] || 'default';
}
</script>

<template>
  <div>
    <Grid>
      <template #toolbar-tools>
        <a-button type="primary" @click="handleCreate">
          新建托管者
        </a-button>
      </template>

      <template #type="{ row }">
        <a-tag :color="row.type === 1 ? 'blue' : 'orange'">
          {{ getServerTypeText(row.type) }}
        </a-tag>
      </template>

      <template #status="{ row }">
        <a-tag :color="getServerStatusColor(row.status)">
          {{ getServerStatusText(row.status) }}
        </a-tag>
      </template>

      <template #action="{ row }">
        <a-space>
          <a-button
            size="small"
            type="link"
            @click="handleEdit(row)"
          >
            <template #icon>
              <component :is="h(LucideFilePenLine)" />
            </template>
          </a-button>
          <a-button
            size="small"
            type="link"
            @click="handleReboot(row)"
          >
            <template #icon>
              <component :is="h(LucideRefreshCw)" />
            </template>
          </a-button>
          <a-button
            size="small"
            type="link"
            @click="handleViewLog(row)"
          >
            <template #icon>
              <component :is="h(LucideFileText)" />
            </template>
          </a-button>
          <a-popconfirm
            title="确定要删除这个托管者吗？"
            @confirm="handleDelete(row)"
          >
            <a-button
              danger
              size="small"
              type="link"
            >
              <template #icon>
                <component :is="h(LucideTrash2)" />
              </template>
            </a-button>
          </a-popconfirm>
        </a-space>
      </template>
    </Grid>

    <Drawer />
  </div>
</template>
