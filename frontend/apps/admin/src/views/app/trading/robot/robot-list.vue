<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';

import { h, onMounted } from 'vue';

import { useVbenDrawer, type VbenFormProps } from '@vben/common-ui';
import { LucideFilePenLine, LucideTrash2, LucideRefreshCw } from '@vben/icons';

import { notification } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  robotStatusList,
  robotStatusToColor,
  robotStatusToName,
  type RobotInfo,
  useRobotStore,
} from '#/stores/robot.state';

import RobotDrawer from './robot-drawer.vue';

const robotStore = useRobotStore();

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'robotId',
      label: '机器人ID',
      componentProps: {
        placeholder: '请输入机器人ID',
        allowClear: true,
      },
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
      component: 'Input',
      fieldName: 'exchange',
      label: '交易所',
      componentProps: {
        placeholder: '请输入交易所',
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'status',
      label: '状态',
      componentProps: {
        placeholder: '请选择状态',
        allowClear: true,
        options: robotStatusList.value,
      },
    },
  ],
};

const gridOptions: VxeGridProps<RobotInfo> = {
  height: 'auto',
  stripe: true,
  toolbarConfig: {
    custom: false,
    export: true,
    refresh: true,
    zoom: false,
  },
  exportConfig: {},
  pagerConfig: {
    enabled: false,
  },
  rowConfig: {
    isHover: true,
  },

  proxyConfig: {
    ajax: {
      query: async (_params, formValues) => {
        const result = await robotStore.listRobots();
        let items = result.items || [];

        // 前端过滤
        if (formValues) {
          if (formValues.robotId) {
            items = items.filter((item) =>
              item.robotId
                ?.toLowerCase()
                .includes(formValues.robotId.toLowerCase()),
            );
          }
          if (formValues.nickname) {
            items = items.filter((item) =>
              item.nickname
                ?.toLowerCase()
                .includes(formValues.nickname.toLowerCase()),
            );
          }
          if (formValues.exchange) {
            items = items.filter((item) =>
              item.exchange
                ?.toLowerCase()
                .includes(formValues.exchange.toLowerCase()),
            );
          }
          if (formValues.status) {
            items = items.filter((item) => item.status === formValues.status);
          }
        }

        return {
          total: items.length,
          items,
        };
      },
    },
  },

  columns: [
    { title: '机器人ID', field: 'robotId', minWidth: 150 },
    { title: '昵称', field: 'nickname', minWidth: 120 },
    { title: '交易所', field: 'exchange', minWidth: 100 },
    { title: '版本', field: 'version', width: 100 },
    {
      title: '状态',
      field: 'status',
      width: 100,
      slots: { default: 'status' },
    },
    {
      title: '初始资金',
      field: 'initBalance',
      width: 120,
      formatter: ({ cellValue }) => cellValue?.toFixed(2) ?? '-',
    },
    {
      title: '余额',
      field: 'balance',
      width: 120,
      formatter: ({ cellValue }) => cellValue?.toFixed(2) ?? '-',
    },
    {
      title: '注册时间',
      field: 'registeredAt',
      width: 160,
      formatter: 'formatDateTime',
    },
    {
      title: '最后心跳',
      field: 'lastHeartbeat',
      width: 160,
      formatter: 'formatDateTime',
    },
    {
      title: '操作',
      field: 'action',
      fixed: 'right',
      slots: { default: 'action' },
      width: 120,
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
  connectedComponent: RobotDrawer,

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
    await robotStore.deleteRobot(row.robotId);
    notification.success({
      message: '删除成功',
      description: `Robot ${row.robotId} 已删除`,
    });
    gridApi.reload();
  } catch (error: any) {
    notification.error({
      message: '删除失败',
      description: error.message || '删除Robot时发生错误',
    });
  }
}

// 手动刷新
function handleRefresh() {
  gridApi.reload();
}

onMounted(() => {
  // 初始加载数据
  gridApi.reload();
});
</script>

<template>
  <div class="h-full">
    <Grid table-title="Robot 列表">
      <template #toolbar-tools>
        <a-space>
          <a-tag color="blue">
            在线: {{ robotStore.onlineRobotCount }} / {{ robotStore.robotCount }}
          </a-tag>
          <a-button type="primary" @click="handleCreate">
            新建Robot
          </a-button>
          <a-button @click="handleRefresh">
            <template #icon>
              <component :is="h(LucideRefreshCw)" />
            </template>
            刷新
          </a-button>
        </a-space>
      </template>

      <template #status="{ row }">
        <a-tag :color="robotStatusToColor(row.status)">
          {{ robotStatusToName(row.status) }}
        </a-tag>
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
          <a-tooltip title="删除">
            <a-popconfirm
              title="确定要删除这个Robot吗？"
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

<style lang="less" scoped>

</style>
