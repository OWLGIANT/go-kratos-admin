<script lang="ts" setup>
import type { VxeGridListeners, VxeGridProps } from '#/adapter/vxe-table';

import { h } from 'vue';

import { useVbenDrawer, type VbenFormProps } from '@vben/common-ui';
import { LucideFilePenLine, LucideTrash2 } from '@vben/icons';

import { notification } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { $t } from '#/locales';
import { useExchangeAccountStore } from '#/stores/exchange-account.state';

import ExchangeAccountDrawer from './exchange-account-drawer.vue';

const exchangeAccountStore = useExchangeAccountStore();

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'nickname',
      label: '账号昵称',
      componentProps: {
        placeholder: '请输入账号昵称',
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'exchange_name',
      label: '交易所',
      componentProps: {
        placeholder: '请输入交易所名称',
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'account_type',
      label: '账号类型',
      componentProps: {
        placeholder: '请选择账号类型',
        allowClear: true,
        options: [
          { label: '自建', value: 1 },
          { label: '平台', value: 2 },
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
        const result = await exchangeAccountStore.listExchangeAccount(
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
    { title: '账号昵称', field: 'nickname', minWidth: 150 },
    { title: '交易所', field: 'exchangeName', width: 120 },
    { title: '原始账号', field: 'originAccount', minWidth: 150 },
    { title: 'API Key', field: 'apiKey', minWidth: 200 },
    { title: '经纪商ID', field: 'brokerId', width: 120 },
    {
      title: '账号类型',
      field: 'accountType',
      width: 100,
      slots: { default: 'accountType' },
    },
    {
      title: '组合账号',
      field: 'isMulti',
      width: 100,
      slots: { default: 'isMulti' },
    },
    { title: '绑定托管者IP', field: 'serverIps', minWidth: 150 },
    { title: '特殊限频', field: 'specialReqLimit', width: 100 },
    { title: '备注', field: 'remark', minWidth: 150 },
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
  connectedComponent: ExchangeAccountDrawer,

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
    await exchangeAccountStore.deleteExchangeAccount(row.id);
    notification.success({
      message: '删除成功',
      description: `账号 ${row.nickname} 已删除`,
    });
    gridApi.reload();
  } catch (error: any) {
    notification.error({
      message: '删除失败',
      description: error.message || '删除账号时发生错误',
    });
  }
}

function getAccountTypeText(type: number) {
  return type === 1 ? '自建' : '平台';
}
</script>

<template>
  <div>
    <Grid>
      <template #toolbar-tools>
        <a-button type="primary" @click="handleCreate">
          新建账号
        </a-button>
      </template>

      <template #accountType="{ row }">
        <a-tag :color="row.accountType === 1 ? 'blue' : 'green'">
          {{ getAccountTypeText(row.accountType) }}
        </a-tag>
      </template>

      <template #isMulti="{ row }">
        <a-tag :color="row.isMulti ? 'orange' : 'default'">
          {{ row.isMulti ? '是' : '否' }}
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
          <a-popconfirm
            title="确定要删除这个账号吗？"
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
