import { defineStore } from 'pinia';

import {
  createOssServiceClient,
  type fileservicev1_GetUploadPresignedUrlRequest as GetUploadPresignedUrlRequest,
} from '#/generated/api/admin/service/v1';
import { requestClientRequestHandler } from '#/utils/request';

export const useOssStore = defineStore('oss', () => {
  const service = createOssServiceClient(requestClientRequestHandler);

  async function getUploadPresignedUrl(values: GetUploadPresignedUrlRequest) {
    return await service.GetUploadPresignedUrl(values);
  }

  function $reset() {}

  return {
    $reset,
    getUploadPresignedUrl,
  };
});
