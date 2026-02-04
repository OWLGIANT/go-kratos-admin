import { computed, ref } from 'vue';

import { defineStore } from 'pinia';

import {
  createActorServiceClient,
  type tradingservicev1_Actor,
} from '#/generated/api/admin/service/v1';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

// Actor 状态类型
export type ActorStatus = 'connected' | 'disconnected' | 'error' | 'idle';

// Actor 信息接口（服务器信息）
export interface ActorInfo {
  clientId: string;
  status: ActorStatus;
  registeredAt: string;
  lastHeartbeat: string;
  // Server 相关信息
  serverInfo?: {
    cpu?: string;
    ipPool?: number;
    mem?: number;
    memPct?: string;
    diskPct?: string;
    taskNum?: number;
    straVersion?: boolean;
    straVersionDetail?: Record<string, string>;
    awsAcct?: string;
    awsZone?: string;
  };
  ip?: string;
  innerIp?: string;
  port?: string;
  machineId?: string;
  nickname?: string;
}

export const useActorStore = defineStore('actor', () => {
  const service = createActorServiceClient(requestClientRequestHandler);

  // Actor 列表
  const actors = ref<ActorInfo[]>([]);

  // 是否正在加载
  const loading = ref(false);

  /**
   * 从 API 响应转换为 ActorInfo
   */
  function convertToActorInfo(actor: tradingservicev1_Actor): ActorInfo {
    return {
      clientId: actor.clientId || '',
      status: (actor.status as ActorStatus) || 'disconnected',
      registeredAt: actor.registeredAt || '',
      lastHeartbeat: actor.lastHeartbeat || '',
      serverInfo: actor.serverInfo
        ? {
            cpu: actor.serverInfo.cpu,
            ipPool: actor.serverInfo.ipPool,
            mem: actor.serverInfo.mem,
            memPct: actor.serverInfo.memPct,
            diskPct: actor.serverInfo.diskPct,
            taskNum: actor.serverInfo.taskNum,
            straVersion: actor.serverInfo.straVersion,
            straVersionDetail: actor.serverInfo.straVersionDetail,
            awsAcct: actor.serverInfo.awsAcct,
            awsZone: actor.serverInfo.awsZone,
          }
        : undefined,
      ip: actor.ip,
      innerIp: actor.innerIp,
      port: actor.port,
      machineId: actor.machineId,
      nickname: actor.nickname,
    };
  }

  /**
   * 获取 Actor 列表
   * 通过 HTTP API 获取数据
   */
  async function listActors(paging?: Paging) {
    loading.value = true;
    try {
      const noPaging =
        paging?.page === undefined && paging?.pageSize === undefined;
      const response = await service.ListActor({
        page: paging?.page,
        pageSize: paging?.pageSize,
        noPaging,
      });

      // 转换并更新本地状态
      const items = (response.items || []).map(convertToActorInfo);
      actors.value = items;

      return {
        total: response.total || 0,
        items,
      };
    } finally {
      loading.value = false;
    }
  }

  /**
   * 获取单个 Actor
   */
  async function getActor(robotId: string) {
    const response = await service.GetActor({ robotId });
    return response ? convertToActorInfo(response) : null;
  }

  /**
   * 更新 Actor 列表
   * 由 WebSocket 消息触发
   */
  function updateActors(newActors: ActorInfo[]) {
    actors.value = newActors;
  }

  /**
   * 添加或更新单个 Actor
   */
  function upsertActor(actor: ActorInfo) {
    const index = actors.value.findIndex((a) => a.clientId === actor.clientId);
    if (index >= 0) {
      actors.value[index] = actor;
    } else {
      actors.value.push(actor);
    }
  }

  /**
   * 移除 Actor
   */
  function removeActor(clientId: string) {
    const index = actors.value.findIndex((a) => a.clientId === clientId);
    if (index >= 0) {
      actors.value.splice(index, 1);
    }
  }

  /**
   * 清空 Actor 列表
   */
  function clearActors() {
    actors.value = [];
  }

  /**
   * 获取 Actor 数量
   */
  const actorCount = computed(() => actors.value.length);

  /**
   * 获取在线 Actor 数量
   */
  const onlineActorCount = computed(
    () => actors.value.filter((a) => a.status === 'connected').length,
  );

  function $reset() {
    actors.value = [];
    loading.value = false;
  }

  return {
    actors,
    loading,
    actorCount,
    onlineActorCount,
    listActors,
    getActor,
    updateActors,
    upsertActor,
    removeActor,
    clearActors,
    $reset,
  };
});

// Actor 状态列表
export const actorStatusList = computed(() => [
  {
    value: 'connected',
    label: '在线',
  },
  {
    value: 'disconnected',
    label: '离线',
  },
  {
    value: 'idle',
    label: '空闲',
  },
  {
    value: 'error',
    label: '错误',
  },
]);

/**
 * Actor 状态转名称
 */
export function actorStatusToName(status: ActorStatus) {
  const values = actorStatusList.value;
  const matchedItem = values.find((item) => item.value === status);
  return matchedItem ? matchedItem.label : '未知';
}

/**
 * Actor 状态转颜色
 */
export function actorStatusToColor(status: ActorStatus) {
  switch (status) {
    case 'connected': {
      return '#52C41A'; // 绿色 - 在线
    }
    case 'disconnected': {
      return '#8C8C8C'; // 灰色 - 离线
    }
    case 'idle': {
      return '#1890FF'; // 蓝色 - 空闲
    }
    case 'error': {
      return '#F5222D'; // 红色 - 错误
    }
    default: {
      return '#C9CDD4'; // 默认灰色
    }
  }
}
