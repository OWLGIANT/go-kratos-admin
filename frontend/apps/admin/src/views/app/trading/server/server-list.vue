<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';

import { h } from 'vue';

import { useVbenDrawer, type VbenFormProps } from '@vben/common-ui';
import { LucideFilePenLine, LucideTrash2, LucideRefreshCw, LucideFileText } from '@vben/icons';

import { notification, Modal } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
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
      label: '昵称',
      componentProps: {
        placeholder: '请输入托管者昵称',
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'ip',
      label: '外网IP',
      componentProps: {
        placeholder: '请输入外网IP',
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'inner_ip',
      label: '内网IP',
      componentProps: {
        placeholder: '请输入内网IP',
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'type',
      label: '类型',
      componentProps: {
        placeholder: '请选择类型',
        allowClear: true,
        options: [
          { label: '自建', value: 'SERVER_TYPE_SELF_BUILT' },
          { label: '平台', value: 'SERVER_TYPE_PLATFORM' },
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
          total: Number(result.total || 0),
          items: result.items || [],
        };
      },
    },
  },

  columns: [
    { title: 'ID', field: 'id', width: 80 },
    { title: '昵称', field: 'nickname', minWidth: 120 },
    { title: '外网IP', field: 'ip', minWidth: 140 },
    { title: '内网IP', field: 'innerIp', minWidth: 140 },
    { title: '端口', field: 'port', width: 80 },
    { title: '机器ID', field: 'machineId', minWidth: 150 },
    { title: 'VPC ID', field: 'vpcId', minWidth: 120 },
    { title: '实例ID', field: 'instanceId', minWidth: 150 },
    {
      title: '类型',
      field: 'type',
      width: 100,
      slots: { default: 'type' },
    },
    {
      title: 'CPU',
      field: 'serverInfo',
      width: 80,
      slots: { default: 'cpu' },
    },
    {
      title: '内存',
      field: 'serverInfo',
      width: 80,
      slots: { default: 'memPct' },
    },
    {
      title: '磁盘',
      field: 'serverInfo',
      width: 80,
      slots: { default: 'diskPct' },
    },
    {
      title: '任务数',
      field: 'serverInfo',
      width: 80,
      slots: { default: 'taskNum' },
    },
    { title: '操作员', field: 'operator', width: 100 },
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
    const result = await serverStore.getServerLog(row.id, 200);
    Modal.info({
      title: `服务器日志 - ${row.nickname}`,
      content: h('pre', {
        style: {
          maxHeight: '400px',
          overflow: 'auto',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
          fontSize: '12px',
          backgroundColor: '#f5f5f5',
          padding: '10px',
          borderRadius: '4px',
        },
      }, result.logContent || '暂无日志'),
      width: 800,
      okText: '关闭',
    });
  } catch (error: any) {
    notification.error({
      message: '获取日志失败',
      description: error.message || '获取日志时发生错误',
    });
  }
}

function getTypeText(type: string) {
  switch (type) {
    case 'SERVER_TYPE_SELF_BUILT':
      return '自建';
    case 'SERVER_TYPE_PLATFORM':
      return '平台';
    default:
      return '未知';
  }
}

function getTypeColor(type: string) {
  switch (type) {
    case 'SERVER_TYPE_SELF_BUILT':
      return 'blue';
    case 'SERVER_TYPE_PLATFORM':
      return 'green';
    default:
      return 'default';
  }
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
        <a-tag :color="getTypeColor(row.type)">
          {{ getTypeText(row.type) }}
        </a-tag>
      </template>

      <template #cpu="{ row }">
        <span>{{ row.serverInfo?.cpu || '-' }}</span>
      </template>

      <template #memPct="{ row }">
        <span>{{ row.serverInfo?.memPct || '-' }}</span>
      </template>

      <template #diskPct="{ row }">
        <span>{{ row.serverInfo?.diskPct || '-' }}</span>
      </template>

      <template #taskNum="{ row }">
        <span>{{ row.serverInfo?.taskNum ?? '-' }}</span>
      </template>

      <template #action="{ row }">
        <a-space>
          <a-tooltip title="编辑">
            <a-button
              size="small"
              type="link"
              @click="handleEdit(row)"
            >
              <template #icon>
                <component :is="h(LucideFilePenLine)" />
              </template>
            </a-button>
          </a-tooltip>
          <a-tooltip title="重启">
            <a-popconfirm
              title="确定要重启这个托管者吗？"
              @confirm="handleReboot(row)"
            >
              <a-button
                size="small"
                type="link"
              >
                <template #icon>
                  <component :is="h(LucideRefreshCw)" />
                </template>
              </a-button>
            </a-popconfirm>
          </a-tooltip>
          <a-tooltip title="查看日志">
            <a-button
              size="small"
              type="link"
              @click="handleViewLog(row)"
            >
              <template #icon>
                <component :is="h(LucideFileText)" />
              </template>
            </a-button>
          </a-tooltip>
          <a-tooltip title="删除">
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
          </a-tooltip>
        </a-space>
      </template>
    </Grid>

    <Drawer />
  </div>
</template>
