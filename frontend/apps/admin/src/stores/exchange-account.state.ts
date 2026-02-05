import { computed, ref } from 'vue';

import { defineStore } from 'pinia';

import {
  createExchangeAccountServiceClient,
  type tradingservicev1_ExchangeAccount,
} from '#/generated/api/admin/service/v1';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

// 账号类型
export type AccountType = 'ACCOUNT_TYPE_UNSPECIFIED' | 'ACCOUNT_TYPE_SELF_BUILT' | 'ACCOUNT_TYPE_PLATFORM';

// 交易账号信息接口
export interface ExchangeAccountInfo {
  id: number;
  nickname?: string;
  exchangeName?: string;
  originAccount?: string;
  apiKey?: string;
  secretKey?: string;
  passKey?: string;
  brokerId?: string;
  operator?: string;
  remark?: string;
  serverIps?: string;
  specialReqLimit?: number;
  accountType?: AccountType;
  applyTime?: number;
  isCombined?: boolean;
  isMulti?: boolean;
  accountIds?: string[];
  motherId?: number;
  createTime?: string;
  updateTime?: string;
}

export const useExchangeAccountStore = defineStore('exchangeAccount', () => {
  const service = createExchangeAccountServiceClient(requestClientRequestHandler);

  // 账号列表
  const accounts = ref<ExchangeAccountInfo[]>([]);

  // 是否正在加载
  const loading = ref(false);

  /**
   * 从 API 响应转换为 ExchangeAccountInfo
   */
  function convertToAccountInfo(account: tradingservicev1_ExchangeAccount): ExchangeAccountInfo {
    return {
      id: account.id || 0,
      nickname: account.nickname,
      exchangeName: account.exchangeName,
      originAccount: account.originAccount,
      apiKey: account.apiKey,
      secretKey: account.secretKey,
      passKey: account.passKey,
      brokerId: account.brokerId,
      operator: account.operator,
      remark: account.remark,
      serverIps: account.serverIps,
      specialReqLimit: account.specialReqLimit,
      accountType: account.accountType as AccountType,
      applyTime: account.applyTime,
      isCombined: account.isCombined,
      isMulti: account.isMulti,
      accountIds: account.accountIds,
      motherId: account.motherId,
      createTime: account.createTime,
      updateTime: account.updateTime,
    };
  }

  /**
   * 获取账号列表
   */
  async function listAccounts(paging?: Paging) {
    loading.value = true;
    try {
      const noPaging =
        paging?.page === undefined && paging?.pageSize === undefined;
      const response = await service.ListExchangeAccount({
        page: paging?.page,
        pageSize: paging?.pageSize,
        noPaging,
      });

      const items = (response.items || []).map(convertToAccountInfo);
      accounts.value = items;

      return {
        total: response.total || 0,
        items,
      };
    } finally {
      loading.value = false;
    }
  }

  /**
   * 获取单个账号
   */
  async function getAccount(id: number) {
    const response = await service.GetExchangeAccount({ id });
    return response ? convertToAccountInfo(response) : null;
  }

  /**
   * 创建账号
   */
  async function createAccount(data: {
    nickname: string;
    exchangeName: string;
    originAccount: string;
    apiKey: string;
    secretKey: string;
    passKey?: string;
    brokerId?: string;
    remark?: string;
    serverIps?: string;
    specialReqLimit?: number;
    accountType?: string;
  }) {
    await service.CreateExchangeAccount(data);
  }

  /**
   * 更新账号
   */
  async function updateAccount(id: number, data: {
    nickname?: string;
    remark?: string;
    serverIps?: string;
    specialReqLimit?: number;
    apiKey?: string;
    secretKey?: string;
    passKey?: string;
    brokerId?: string;
  }) {
    await service.UpdateExchangeAccount({ id, ...data });
  }

  /**
   * 删除账号
   */
  async function deleteAccount(id: number) {
    await service.DeleteExchangeAccount({ id });
  }

  /**
   * 获取账号数量
   */
  const accountCount = computed(() => accounts.value.length);

  function $reset() {
    accounts.value = [];
    loading.value = false;
  }

  return {
    accounts,
    loading,
    accountCount,
    listAccounts,
    getAccount,
    createAccount,
    updateAccount,
    deleteAccount,
    $reset,
  };
});

// 账号类型列表
export const accountTypeList = computed(() => [
  {
    value: 'ACCOUNT_TYPE_SELF_BUILT',
    label: '自建账号',
  },
  {
    value: 'ACCOUNT_TYPE_PLATFORM',
    label: '平台账号',
  },
]);

/**
 * 账号类型转名称
 */
export function accountTypeToName(type?: AccountType) {
  if (!type || type === 'ACCOUNT_TYPE_UNSPECIFIED') return '未知';
  const values = accountTypeList.value;
  const matchedItem = values.find((item) => item.value === type);
  return matchedItem ? matchedItem.label : '未知';
}
