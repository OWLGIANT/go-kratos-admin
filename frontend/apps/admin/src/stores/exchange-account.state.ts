import { defineStore } from 'pinia';

import {
  createExchangeAccountServiceClient,
} from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString, makeUpdateMask } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useExchangeAccountStore = defineStore('exchange-account', () => {
  const service = createExchangeAccountServiceClient(requestClientRequestHandler);

  /**
   * 查询交易账号列表
   */
  async function listExchangeAccount(
    paging?: Paging,
    formValues?: null | object,
    fieldMask?: null | string,
    orderBy?: null | string[],
  ) {
    const noPaging =
      paging?.page === undefined && paging?.pageSize === undefined;
    return await service.ListExchangeAccount({
      // @ts-ignore proto generated code is error.
      fieldMask,
      orderBy: makeOrderBy(orderBy),
      query: makeQueryString(formValues),
      page: paging?.page,
      pageSize: paging?.pageSize,
      noPaging,
    });
  }

  /**
   * 获取交易账号
   */
  async function getExchangeAccount(id: number) {
    return await service.GetExchangeAccount({ id });
  }

  /**
   * 创建交易账号
   */
  async function createExchangeAccount(values: object) {
    return await service.CreateExchangeAccount({
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
    });
  }

  /**
   * 更新交易账号
   */
  async function updateExchangeAccount(id: number, values: object) {
    const updateMask = makeUpdateMask(Object.keys(values ?? []));
    return await service.UpdateExchangeAccount({
      id,
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
      // @ts-ignore proto generated code is error.
      updateMask,
    });
  }

  /**
   * 删除交易账号
   */
  async function deleteExchangeAccount(id: number) {
    return await service.DeleteExchangeAccount({ id });
  }

  /**
   * 批量删除交易账号
   */
  async function batchDeleteExchangeAccount(ids: number[]) {
    return await service.BatchDeleteExchangeAccount({ ids });
  }

  /**
   * 转移交易账号
   */
  async function transferExchangeAccount(ids: number[], targetOperatorId: number) {
    return await service.TransferExchangeAccount({
      ids,
      targetOperatorId
    });
  }

  /**
   * 搜索交易账号
   */
  async function searchExchangeAccount(
    keyword: string,
    paging?: Paging,
  ) {
    return await service.SearchExchangeAccount({
      keyword,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  /**
   * 获取账号资金曲线
   */
  async function getAccountEquity(
    accountId: number,
    startTime?: number,
    endTime?: number,
  ) {
    return await service.GetAccountEquity({
      accountId,
      startTime,
      endTime,
    });
  }

  /**
   * 创建组合账号
   */
  async function createCombinedAccount(values: object) {
    return await service.CreateCombinedAccount({
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
    });
  }

  /**
   * 更新组合账号
   */
  async function updateCombinedAccount(id: number, values: object) {
    return await service.UpdateCombinedAccount({
      id,
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
    });
  }

  /**
   * 更新账号备注
   */
  async function updateAccountRemark(id: number, remark: string) {
    return await service.UpdateAccountRemark({
      id,
      remark,
    });
  }

  /**
   * 更新账号经纪商ID
   */
  async function updateAccountBrokerId(id: number, brokerId: string) {
    return await service.UpdateAccountBrokerId({
      id,
      brokerId,
    });
  }

  return {
    listExchangeAccount,
    getExchangeAccount,
    createExchangeAccount,
    updateExchangeAccount,
    deleteExchangeAccount,
    batchDeleteExchangeAccount,
    transferExchangeAccount,
    searchExchangeAccount,
    getAccountEquity,
    createCombinedAccount,
    updateCombinedAccount,
    updateAccountRemark,
    updateAccountBrokerId,
  };
});
