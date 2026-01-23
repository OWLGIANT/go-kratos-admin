import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import {
  createDictEntryServiceClient,
  createDictTypeServiceClient,
} from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString, makeUpdateMask } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useDictStore = defineStore('dict', () => {
  const dictTypeService = createDictTypeServiceClient(
    requestClientRequestHandler,
  );
  const dictEntryService = createDictEntryServiceClient(
    requestClientRequestHandler,
  );
  const userStore = useUserStore();

  /**
   * 查询字典类型列表
   */
  async function listDictType(
    paging?: Paging,
    formValues?: null | object,
    fieldMask?: null | string,
    orderBy?: null | string[],
  ) {
    const noPaging =
      paging?.page === undefined && paging?.pageSize === undefined;
    return await dictTypeService.ListDictType({
      // @ts-ignore proto generated code is error.
      fieldMask,
      orderBy: makeOrderBy(orderBy),
      query: makeQueryString(formValues, userStore.isTenantUser()),
      page: paging?.page,
      pageSize: paging?.pageSize,
      noPaging,
    });
  }

  /**
   * 查询字典项列表
   */
  async function listDictEntry(
    paging?: Paging,
    formValues?: null | object,
    fieldMask?: null | string,
    orderBy?: null | string[],
  ) {
    const noPaging =
      paging?.page === undefined && paging?.pageSize === undefined;
    return await dictEntryService.ListDictEntry({
      // @ts-ignore proto generated code is error.
      fieldMask,
      orderBy: makeOrderBy(orderBy),
      query: makeQueryString(formValues, userStore.isTenantUser()),
      page: paging?.page,
      pageSize: paging?.pageSize,
      noPaging,
    });
  }

  /**
   * 获取字典类型
   */
  async function getDictType(id: number) {
    return await dictTypeService.GetDictType({
      id,
    });
  }

  /**
   * 获取字典类型
   */
  async function getDictTypeByCode(code: string) {
    return await dictTypeService.GetDictType({
      code,
    });
  }

  /**
   * 创建字典类型
   */
  async function createDictType(values: object) {
    return await dictTypeService.CreateDictType({
      data: {
        ...values,
      },
    });
  }

  /**
   * 创建字典项
   */
  async function createDictEntry(values: object) {
    return await dictEntryService.CreateDictEntry({
      data: {
        ...values,
      },
    });
  }

  /**
   * 更新字典类型
   */
  async function updateDictType(id: number, values: object) {
    return await dictTypeService.UpdateDictType({
      id,
      data: {
        ...values,
      },
      // @ts-ignore proto generated code is error.
      updateMask: makeUpdateMask(Object.keys(values ?? [])),
    });
  }

  /**
   * 更新字典项
   */
  async function updateDictEntry(id: number, values: object) {
    return await dictEntryService.UpdateDictEntry({
      id,
      data: {
        ...values,
      },
      // @ts-ignore proto generated code is error.
      updateMask: makeUpdateMask(Object.keys(values ?? [])),
    });
  }

  /**
   * 删除字典类型
   */
  async function deleteDictType(ids: number[]) {
    return await dictTypeService.DeleteDictType({ ids });
  }

  /**
   * 删除字典项
   */
  async function deleteDictEntry(ids: number[]) {
    return await dictEntryService.DeleteDictEntry({ ids });
  }

  function $reset() {}

  return {
    $reset,
    listDictType,
    listDictEntry,
    getDictType,
    getDictTypeByCode,
    createDictType,
    createDictEntry,
    updateDictType,
    updateDictEntry,
    deleteDictType,
    deleteDictEntry,
  };
});
