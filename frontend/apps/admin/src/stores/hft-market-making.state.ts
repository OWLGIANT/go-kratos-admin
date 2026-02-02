import { defineStore } from 'pinia';

import {
  createHftMarketMakingServiceClient,
} from '#/generated/api/admin/service/v1';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useHftMarketMakingStore = defineStore('hft-market-making', () => {
  const service = createHftMarketMakingServiceClient(requestClientRequestHandler);

  /**
   * 获取 MidSigExec 订单列表
   */
  async function listMidSigExecOrders(
    startTime?: number,
    endTime?: number,
    symbol?: string,
    paging?: Paging,
  ) {
    return await service.ListMidSigExecOrders({
      startTime,
      endTime,
      symbol,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  /**
   * 获取 MidSigExec 信号列表
   */
  async function listMidSigExecSignals(
    startTime?: number,
    endTime?: number,
    symbol?: string,
    paging?: Paging,
  ) {
    return await service.ListMidSigExecSignals({
      startTime,
      endTime,
      symbol,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  /**
   * 获取 MidSigExec 结果列表
   */
  async function listMidSigExecDetails(
    startTime?: number,
    endTime?: number,
    symbol?: string,
    paging?: Paging,
  ) {
    return await service.ListMidSigExecDetails({
      startTime,
      endTime,
      symbol,
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  /**
   * 获取 HFT 信息
   */
  async function getHftInfo(
    startTime?: number,
    endTime?: number,
  ) {
    return await service.GetHftInfo({
      startTime,
      endTime,
    });
  }

  /**
   * 下载 MidSigExec 数据
   */
  async function downloadMidSigExec(
    dataType: string,
    startTime?: number,
    endTime?: number,
    symbol?: string,
  ) {
    return await service.DownloadMidSigExec({
      dataType,
      startTime,
      endTime,
      symbol,
    });
  }

  /**
   * 获取 HFT 通知报告
   */
  async function getHftNotifyReport(
    startTime?: number,
    endTime?: number,
  ) {
    return await service.GetHftNotifyReport({
      startTime,
      endTime,
    });
  }

  return {
    listMidSigExecOrders,
    listMidSigExecSignals,
    listMidSigExecDetails,
    getHftInfo,
    downloadMidSigExec,
    getHftNotifyReport,
  };
});
