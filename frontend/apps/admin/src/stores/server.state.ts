import { defineStore } from 'pinia';

import {
  createServerServiceClient,
} from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString, makeUpdateMask } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useServerStore = defineStore('server', () => {
  const service = createServerServiceClient(requestClientRequestHandler);

  /**
   * 查询托管者列表
   */
  async function listServer(
    paging?: Paging,
    formValues?: null | object,
    fieldMask?: null | string,
    orderBy?: null | string[],
  ) {
    const noPaging =
      paging?.page === undefined && paging?.pageSize === undefined;
    return await service.ListServer({
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
   * 获取托管者
   */
  async function getServer(id: number) {
    return await service.GetServer({ id });
  }

  /**
   * 创建托管者
   */
  async function createServer(values: object) {
    return await service.CreateServer({
      // @ts-ignore proto generated code is error.
      data: {
        ...values,
      },
    });
  }

  /**
   * 批量创建托管者
   */
  async function batchCreateServer(servers: object[]) {
    return await service.BatchCreateServer({
      // @ts-ignore proto generated code is error.
      servers,
    });
  }

  /**
   * 更新托管者
   */
  async function updateServer(id: number, values: object) {
    const updateMask = makeUpdateMask(Object.keys(values ?? []));
    return await service.UpdateServer({
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
   * 删除托管者
   */
  async function deleteServer(id: number) {
    return await service.DeleteServer({ id });
  }

  /**
   * 按IP删除托管者
   */
  async function deleteServerByIps(ips: string[]) {
    return await service.DeleteServerByIps({ ips });
  }

  /**
   * 重启托管者
   */
  async function rebootServer(id: number) {
    return await service.RebootServer({ id });
  }

  /**
   * 获取托管者日志
   */
  async function getServerLog(id: number, lines?: number) {
    return await service.GetServerLog({
      id,
      lines: lines ?? 100,
    });
  }

  /**
   * 停止托管者上的机器人
   */
  async function stopServerRobot(id: number, robotId: string) {
    return await service.StopServerRobot({
      id,
      robotId,
    });
  }

  /**
   * 转移托管者
   */
  async function transferServer(ids: number[], targetOperatorId: number) {
    return await service.TransferServer({
      ids,
      targetOperatorId,
    });
  }

  /**
   * 删除托管者日志
   */
  async function deleteServerLog(id: number) {
    return await service.DeleteServerLog({ id });
  }

  /**
   * 更新托管者策略
   */
  async function updateServerStrategy(id: number, strategy: string) {
    return await service.UpdateServerStrategy({
      id,
      strategy,
    });
  }

  /**
   * 更新托管者备注
   */
  async function updateServerRemark(id: number, remark: string) {
    return await service.UpdateServerRemark({
      id,
      remark,
    });
  }

  /**
   * 获取可重启的托管者列表
   */
  async function getCanRestartServerList(paging?: Paging) {
    return await service.GetCanRestartServerList({
      page: paging?.page,
      pageSize: paging?.pageSize,
    });
  }

  return {
    listServer,
    getServer,
    createServer,
    batchCreateServer,
    updateServer,
    deleteServer,
    deleteServerByIps,
    rebootServer,
    getServerLog,
    stopServerRobot,
    transferServer,
    deleteServerLog,
    updateServerStrategy,
    updateServerRemark,
    getCanRestartServerList,
  };
});
