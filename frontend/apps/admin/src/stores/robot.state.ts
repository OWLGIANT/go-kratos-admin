import { computed, ref } from 'vue';

import { defineStore } from 'pinia';

import {
  createRobotServiceClient,
  type tradingservicev1_Robot,
} from '#/generated/api/admin/service/v1';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

// Robot 状态类型
export type RobotStatus = 'connected' | 'disconnected' | 'error' | 'idle';

// Robot 信息接口
export interface RobotInfo {
  robotId: string;
  nickname?: string;
  exchange?: string;
  version?: string;
  status: RobotStatus;
  initBalance?: number;
  balance?: number;
  registeredAt?: string;
  lastHeartbeat?: string;
  serverId?: number;
  exchangeAccountId?: number;
}

export const useRobotStore = defineStore('robot', () => {
  const service = createRobotServiceClient(requestClientRequestHandler);

  // Robot 列表
  const robots = ref<RobotInfo[]>([]);

  // 是否正在加载
  const loading = ref(false);

  /**
   * 从 API 响应转换为 RobotInfo
   */
  function convertToRobotInfo(robot: tradingservicev1_Robot): RobotInfo {
    return {
      robotId: robot.robotId || '',
      nickname: robot.nickname,
      exchange: robot.exchange,
      version: robot.version,
      status: (robot.status as RobotStatus) || 'disconnected',
      initBalance: robot.initBalance,
      balance: robot.balance,
      registeredAt: robot.registeredAt,
      lastHeartbeat: robot.lastHeartbeat,
      serverId: robot.serverId,
      exchangeAccountId: robot.exchangeAccountId,
    };
  }

  /**
   * 获取 Robot 列表
   */
  async function listRobots(paging?: Paging) {
    loading.value = true;
    try {
      const noPaging =
        paging?.page === undefined && paging?.pageSize === undefined;
      const response = await service.ListRobot({
        page: paging?.page,
        pageSize: paging?.pageSize,
        noPaging,
      });

      const items = (response.items || []).map(convertToRobotInfo);
      robots.value = items;

      return {
        total: response.total || 0,
        items,
      };
    } finally {
      loading.value = false;
    }
  }

  /**
   * 获取单个 Robot
   */
  async function getRobot(robotId: string) {
    const response = await service.GetRobot({ robotId });
    return response ? convertToRobotInfo(response) : null;
  }

  /**
   * 创建 Robot
   */
  async function createRobot(data: {
    robotId: string;
    nickname?: string;
    exchange?: string;
    version?: string;
    status?: string;
    initBalance?: number;
    balance?: number;
    serverId?: number;
    exchangeAccountId?: number;
  }) {
    await service.CreateRobot(data);
  }

  /**
   * 更新 Robot
   */
  async function updateRobot(data: {
    robotId: string;
    nickname?: string;
    exchange?: string;
    version?: string;
    status?: string;
    initBalance?: number;
    balance?: number;
    serverId?: number;
    exchangeAccountId?: number;
  }) {
    await service.UpdateRobot(data);
  }

  /**
   * 删除 Robot
   */
  async function deleteRobot(robotId: string) {
    await service.DeleteRobot({ robotId });
  }

  /**
   * 获取 Robot 数量
   */
  const robotCount = computed(() => robots.value.length);

  /**
   * 获取在线 Robot 数量
   */
  const onlineRobotCount = computed(
    () => robots.value.filter((r) => r.status === 'connected').length,
  );

  function $reset() {
    robots.value = [];
    loading.value = false;
  }

  return {
    robots,
    loading,
    robotCount,
    onlineRobotCount,
    listRobots,
    getRobot,
    createRobot,
    updateRobot,
    deleteRobot,
    $reset,
  };
});

// Robot 状态列表
export const robotStatusList = computed(() => [
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
 * Robot 状态转名称
 */
export function robotStatusToName(status: RobotStatus) {
  const values = robotStatusList.value;
  const matchedItem = values.find((item) => item.value === status);
  return matchedItem ? matchedItem.label : '未知';
}

/**
 * Robot 状态转颜色
 */
export function robotStatusToColor(status: RobotStatus) {
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
