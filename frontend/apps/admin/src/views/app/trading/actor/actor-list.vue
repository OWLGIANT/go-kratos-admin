<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';

import { h, onMounted } from 'vue';

import { type VbenFormProps } from '@vben/common-ui';
import { LucideRefreshCw } from '@vben/icons';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  actorStatusList,
  actorStatusToColor,
  actorStatusToName,
  type ActorInfo,
  useActorStore,
} from '#/stores/actor.state';

const actorStore = useActorStore();

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
        placeholder: '请输入昵称',
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'ip',
      label: 'IP',
      componentProps: {
        placeholder: '请输入IP',
        allowClear: true,
      },
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
      component: 'Select',
      fieldName: 'status',
      label: '状态',
      componentProps: {
        placeholder: '请选择状态',
        allowClear: true,
        options: actorStatusList.value,
      },
    },
  ],
};

const gridOptions: VxeGridProps<ActorInfo> = {
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
        const result = await actorStore.listActors();
        let items = result.items || [];

        // 前端过滤
        if (formValues) {
          if (formValues.nickname) {
            items = items.filter((item) =>
              item.nickname
                ?.toLowerCase()
                .includes(formValues.nickname.toLowerCase()),
            );
          }
          if (formValues.ip) {
            items = items.filter(
              (item) =>
                item.ip?.toLowerCase().includes(formValues.ip.toLowerCase()) ||
                item.innerIp
                  ?.toLowerCase()
                  .includes(formValues.ip.toLowerCase()),
            );
          }
          if (formValues.machineId) {
            items = items.filter((item) =>
              item.machineId
                ?.toLowerCase()
                .includes(formValues.machineId.toLowerCase()),
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
    { title: '昵称', field: 'nickname', minWidth: 120 },
    { title: '外网IP', field: 'ip', minWidth: 140 },
    { title: '内网IP', field: 'innerIp', minWidth: 140 },
    { title: '端口', field: 'port', width: 80 },
    { title: '机器ID', field: 'machineId', minWidth: 150 },
    {
      title: '状态',
      field: 'status',
      width: 100,
      slots: { default: 'status' },
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
    {
      title: '创建时间',
      field: 'createTime',
      width: 160,
      formatter: 'formatDateTime',
    },
    {
      title: '更新时间',
      field: 'updateTime',
      width: 160,
      formatter: 'formatDateTime',
    },
  ],
};

const gridEvents: VxeGridListeners = {};

const [Grid, gridApi] = useVbenVxeGrid({
  gridOptions,
  formOptions,
  gridEvents,
});

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
    <Grid table-title="Actor 列表">
      <template #toolbar-tools>
        <a-space>
          <a-tag color="blue">
            在线: {{ actorStore.onlineActorCount }} / {{ actorStore.actorCount }}
          </a-tag>
          <a-button type="primary" @click="handleRefresh">
            <template #icon>
              <component :is="h(LucideRefreshCw)" />
            </template>
            刷新
          </a-button>
        </a-space>
      </template>

      <template #status="{ row }">
        <a-tag :color="actorStatusToColor(row.status)">
          {{ actorStatusToName(row.status) }}
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
    </Grid>
  </div>
</template>

<style lang="less" scoped>

</style>
