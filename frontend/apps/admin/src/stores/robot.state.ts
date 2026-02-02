import { defineStore } from 'pinia';
import { ref } from 'vue';
import { message } from 'ant-design-vue';
import type {
  Robot,
  RobotFormData,
  FilterConditions,
  BatchModifyParams,
  CopyParamsRequest,
  StatusHistoryEntry,
  EquityDataPoint,
} from '../views/app/hft-robots/types';
import { MOCK_STRATEGIES, getStrategyDefaultParams } from '../views/app/hft-robots/config';

// Generate mock equity data
function generateEquityData(baseEquity: number, days: number = 7): EquityDataPoint[] {
  const data: EquityDataPoint[] = [];
  const now = Date.now();
  const pointsPerDay = 24; // One point per hour

  for (let i = days * pointsPerDay; i >= 0; i--) {
    const timestamp = now - i * 3600000; // 1 hour intervals
    const randomVariation = (Math.random() - 0.5) * 0.02; // ±1% random variation
    const trend = (days * pointsPerDay - i) * 0.0001; // Slight upward trend
    const equity = baseEquity * (1 + randomVariation + trend);

    data.push({
      time: new Date(timestamp).toLocaleString('zh-CN', {
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
      }),
      equity: Number(equity.toFixed(2)),
      timestamp,
    });
  }

  return data;
}

// Generate mock robots with rich data
function generateMockRobots(): Robot[] {
  const creators = ['admin', 'trader1', 'trader2', 'system'];
  const accounts = ['Account_A', 'Account_B', 'Account_C', 'Account_D'];
  const servers = ['Server-HK-01', 'Server-SG-02', 'Server-JP-03', 'Server-US-04'];

  return [
    {
      id: 1,
      nickname: 'BTC主力做市',
      account: accounts[0],
      exchange: 'okx_spot',
      exchangeName: 'OKX现货',
      pair: 'BTC/USDT',
      strategy: 'match',
      strategyVersion: 'v2.0.0',
      strategyParams: {
        spread: 0.002,
        volume: 100,
        max_position: 10000,
        order_interval: 1000,
        enable_hedge: true,
        price_offset: 0,
      },
      status: 'running',
      currentBalance: 50250.75,
      yield: 2.51,
      volume: 1250000.50,
      cash: 25000.00,
      coin: 0.5,
      equityData: generateEquityData(50250.75),
      makerFee: 0.0002,
      takerFee: 0.0005,
      createTime: '2024-01-15 10:30:00',
      updateTime: '2024-01-31 15:20:00',
      lastResetTime: '2024-01-15 10:30:00',
      lastRunTime: '2024-01-31 08:00:00',
      newDelay: 12,
      cancelDelay: 8,
      systemDelay: 5,
      server: servers[0],
      creator: creators[0],
      remark: '主力机器人，稳定运行',
      isPinned: true,
      tags: ['主力', '稳定'],
    },
    {
      id: 2,
      nickname: 'ETH网格策略',
      account: accounts[1],
      exchange: 'bybit_spot',
      exchangeName: 'Bybit现货',
      pair: 'ETH/USDT',
      strategy: 'grid',
      strategyVersion: 'v2.0.0',
      strategyParams: {
        grid_num: 10,
        price_range: 5.0,
        order_amount: 100,
        profit_ratio: 0.01,
        stop_loss_ratio: 0.05,
        grid_mode: 'arithmetic',
      },
      status: 'running',
      currentBalance: 29680.25,
      yield: -1.07,
      volume: 850000.00,
      cash: 15000.00,
      coin: 5.2,
      equityData: generateEquityData(29680.25),
      makerFee: 0.0001,
      takerFee: 0.0004,
      createTime: '2024-01-16 14:20:00',
      updateTime: '2024-01-31 15:18:00',
      lastRunTime: '2024-01-30 09:00:00',
      newDelay: 15,
      cancelDelay: 10,
      systemDelay: 6,
      server: servers[1],
      creator: creators[1],
      remark: '测试网格策略效果',
      isPinned: false,
      tags: ['测试'],
    },
    {
      id: 3,
      nickname: 'SOL双边做市',
      account: accounts[2],
      exchange: 'gate_spot',
      exchangeName: 'Gate现货',
      pair: 'SOL/USDT',
      strategy: 'dual_match',
      strategyVersion: 'v1.2.0',
      strategyParams: {
        bid_spread: 0.001,
        ask_spread: 0.001,
        order_size: 50,
        refresh_interval: 500,
        inventory_target: 0,
        max_inventory: 5000,
        skew_enabled: true,
      },
      status: 'stopped',
      currentBalance: 15580.80,
      yield: 3.87,
      volume: 420000.00,
      cash: 8000.00,
      coin: 150.5,
      equityData: generateEquityData(15580.80),
      makerFee: 0.0002,
      takerFee: 0.0005,
      createTime: '2024-01-18 09:15:00',
      updateTime: '2024-01-31 12:00:00',
      lastRunTime: '2024-01-30 10:00:00',
      lastStopTime: '2024-01-31 12:00:00',
      newDelay: 18,
      cancelDelay: 12,
      systemDelay: 7,
      server: servers[2],
      creator: creators[2],
      remark: '手动停止进行参数调整',
      isPinned: false,
    },
    {
      id: 4,
      nickname: 'BNB_TWAP执行',
      account: accounts[0],
      exchange: 'binance_spot',
      exchangeName: 'Binance现货',
      pair: 'BNB/USDT',
      strategy: 'twap',
      strategyVersion: 'v1.1.0',
      strategyParams: {
        total_amount: 1000,
        duration: 60,
        slice_num: 20,
        price_limit: 0.5,
        randomize: false,
        side: 'buy',
      },
      status: 'running',
      currentBalance: 25890.40,
      yield: 3.56,
      volume: 680000.00,
      cash: 12000.00,
      coin: 45.2,
      equityData: generateEquityData(25890.40),
      makerFee: 0.0001,
      takerFee: 0.0004,
      createTime: '2024-01-20 16:45:00',
      updateTime: '2024-01-31 15:10:00',
      lastRunTime: '2024-01-31 06:00:00',
      newDelay: 10,
      cancelDelay: 7,
      systemDelay: 4,
      server: servers[0],
      creator: creators[0],
      remark: 'TWAP策略测试',
      isPinned: true,
      tags: ['TWAP', '测试'],
    },
    {
      id: 5,
      nickname: 'DOGE做市_备用',
      account: accounts[3],
      exchange: 'okx_spot',
      exchangeName: 'OKX现货',
      pair: 'DOGE/USDT',
      strategy: 'match',
      strategyVersion: 'v1.2.0',
      strategyParams: {
        spread: 0.003,
        volume: 50,
        max_position: 5000,
        order_interval: 1500,
        enable_hedge: false,
        price_offset: 0.0001,
      },
      status: 'error',
      currentBalance: 7850.00,
      yield: -2.19,
      volume: 180000.00,
      cash: 4000.00,
      coin: 8500.0,
      equityData: generateEquityData(7850.00),
      makerFee: 0.0002,
      takerFee: 0.0005,
      createTime: '2024-01-22 11:30:00',
      updateTime: '2024-01-31 14:45:00',
      lastRunTime: '2024-01-31 14:30:00',
      lastStopTime: '2024-01-31 14:45:00',
      newDelay: 25,
      cancelDelay: 18,
      systemDelay: 12,
      server: servers[3],
      exitMsg: 'WebSocket连接断开: Connection timeout after 30s',
      creator: creators[3],
      remark: '需要检查网络连接',
      isPinned: false,
      tags: ['错误', '待修复'],
    },
    {
      id: 6,
      nickname: 'ADA网格测试',
      account: accounts[1],
      exchange: 'bybit_spot',
      exchangeName: 'Bybit现货',
      pair: 'ADA/USDT',
      strategy: 'grid',
      strategyVersion: 'v1.1.0',
      strategyParams: {
        grid_num: 15,
        price_range: 3.0,
        order_amount: 80,
        profit_ratio: 0.008,
        stop_loss_ratio: 0.04,
        grid_mode: 'geometric',
      },
      status: 'running',
      currentBalance: 12450.60,
      yield: 1.82,
      volume: 320000.00,
      cash: 6000.00,
      coin: 15000.0,
      equityData: generateEquityData(12450.60),
      makerFee: 0.0001,
      takerFee: 0.0004,
      createTime: '2024-01-25 08:00:00',
      updateTime: '2024-01-31 15:15:00',
      lastRunTime: '2024-01-31 07:00:00',
      newDelay: 14,
      cancelDelay: 9,
      systemDelay: 5,
      server: servers[1],
      creator: creators[1],
      remark: '小币种网格测试',
      isPinned: false,
    },
    {
      id: 7,
      nickname: 'XRP双边高频',
      account: accounts[2],
      exchange: 'gate_spot',
      exchangeName: 'Gate现货',
      pair: 'XRP/USDT',
      strategy: 'dual_match',
      strategyVersion: 'v1.2.0',
      strategyParams: {
        bid_spread: 0.0008,
        ask_spread: 0.0008,
        order_size: 100,
        refresh_interval: 300,
        inventory_target: 0,
        max_inventory: 8000,
        skew_enabled: true,
      },
      status: 'running',
      currentBalance: 18920.30,
      yield: 5.12,
      volume: 950000.00,
      cash: 9000.00,
      coin: 18000.0,
      equityData: generateEquityData(18920.30),
      makerFee: 0.0002,
      takerFee: 0.0005,
      createTime: '2024-01-26 13:20:00',
      updateTime: '2024-01-31 15:22:00',
      lastRunTime: '2024-01-31 05:00:00',
      newDelay: 11,
      cancelDelay: 8,
      systemDelay: 4,
      server: servers[2],
      creator: creators[2],
      remark: '高频策略表现良好',
      isPinned: false,
      tags: ['高频', '优秀'],
    },
    {
      id: 8,
      nickname: 'MATIC做市',
      account: accounts[0],
      exchange: 'binance_spot',
      exchangeName: 'Binance现货',
      pair: 'MATIC/USDT',
      strategy: 'match',
      strategyVersion: 'v2.0.0',
      strategyParams: {
        spread: 0.0025,
        volume: 200,
        max_position: 15000,
        order_interval: 800,
        enable_hedge: true,
        price_offset: 0,
      },
      status: 'stopped',
      currentBalance: 22100.00,
      yield: 0.45,
      volume: 560000.00,
      cash: 11000.00,
      coin: 12000.0,
      equityData: generateEquityData(22100.00),
      makerFee: 0.0001,
      takerFee: 0.0004,
      createTime: '2024-01-28 10:00:00',
      updateTime: '2024-01-31 11:30:00',
      lastRunTime: '2024-01-30 08:00:00',
      lastStopTime: '2024-01-31 11:30:00',
      newDelay: 13,
      cancelDelay: 9,
      systemDelay: 5,
      server: servers[0],
      creator: creators[0],
      remark: '暂停观察市场',
      isPinned: false,
    },
  ];
}

export const useRobotStore = defineStore('robot', () => {
  const robotList = ref<Robot[]>([]);
  const loading = ref(false);
  const total = ref(0);
  const filterConditions = ref<FilterConditions>({});

  // Initialize with mock data
  const initMockData = () => {
    robotList.value = generateMockRobots();
    total.value = robotList.value.length;
  };

  // Get robot list with optional filtering
  const getRobotList = async (params?: any) => {
    loading.value = true;
    try {
      await new Promise((resolve) => setTimeout(resolve, 500));

      if (robotList.value.length === 0) {
        initMockData();
      }

      // Apply filters
      let filtered = [...robotList.value];

      if (filterConditions.value.exchange) {
        filtered = filtered.filter((r) => r.exchange === filterConditions.value.exchange);
      }
      if (filterConditions.value.pair) {
        filtered = filtered.filter((r) => r.pair === filterConditions.value.pair);
      }
      if (filterConditions.value.status) {
        filtered = filtered.filter((r) => r.status === filterConditions.value.status);
      }
      if (filterConditions.value.strategy) {
        filtered = filtered.filter((r) => r.strategy === filterConditions.value.strategy);
      }
      if (filterConditions.value.creator) {
        filtered = filtered.filter((r) => r.creator === filterConditions.value.creator);
      }
      if (filterConditions.value.keyword) {
        const keyword = filterConditions.value.keyword.toLowerCase();
        filtered = filtered.filter(
          (r) =>
            r.nickname.toLowerCase().includes(keyword) ||
            r.account.toLowerCase().includes(keyword),
        );
      }

      // Sort: pinned first, then by ID
      filtered.sort((a, b) => {
        if (a.isPinned && !b.isPinned) return -1;
        if (!a.isPinned && b.isPinned) return 1;
        return b.id - a.id;
      });

      return { success: true, data: { list: filtered, total: filtered.length } };
    } catch (error) {
      message.error('获取机器人列表失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // Create robot
  const createRobot = async (formData: RobotFormData) => {
    loading.value = true;
    try {
      await new Promise((resolve) => setTimeout(resolve, 500));

      const newId = Math.max(...robotList.value.map((r) => r.id), 0) + 1;
      const baseEquity = 10000;

      const newRobot: Robot = {
        id: newId,
        nickname: formData.nickname,
        account: formData.account,
        exchange: formData.exchange,
        exchangeName: formData.exchange.replace('_', ' ').toUpperCase(),
        pair: formData.pair,
        strategy: formData.strategy,
        strategyVersion: formData.strategyVersion,
        strategyParams: formData.strategyParams,
        status: 'stopped',
        currentBalance: baseEquity,
        yield: 0,
        volume: 0,
        cash: baseEquity / 2,
        coin: 0,
        equityData: generateEquityData(baseEquity, 1),
        makerFee: 0.0002,
        takerFee: 0.0005,
        createTime: new Date().toLocaleString('zh-CN'),
        updateTime: new Date().toLocaleString('zh-CN'),
        newDelay: 10,
        cancelDelay: 8,
        systemDelay: 5,
        server: 'Server-HK-01',
        creator: 'admin',
        remark: formData.remark,
        isPinned: false,
        tags: [],
      };

      robotList.value.unshift(newRobot);
      total.value = robotList.value.length;

      message.success('创建机器人成功');
      return { success: true, data: newRobot };
    } catch (error) {
      message.error('创建机器人失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // Update robot
  const updateRobot = async (id: number, formData: RobotFormData) => {
    loading.value = true;
    try {
      await new Promise((resolve) => setTimeout(resolve, 500));

      const robot = robotList.value.find((r) => r.id === id);
      if (robot) {
        robot.nickname = formData.nickname;
        robot.account = formData.account;
        robot.exchange = formData.exchange;
        robot.exchangeName = formData.exchange.replace('_', ' ').toUpperCase();
        robot.pair = formData.pair;
        robot.strategy = formData.strategy;
        robot.strategyVersion = formData.strategyVersion;
        robot.strategyParams = formData.strategyParams;
        robot.remark = formData.remark;
        robot.updateTime = new Date().toLocaleString('zh-CN');
      }

      message.success('更新机器人成功');
      return { success: true };
    } catch (error) {
      message.error('更新机器人失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // Delete robot
  const deleteRobot = async (id: number) => {
    loading.value = true;
    try {
      await new Promise((resolve) => setTimeout(resolve, 300));

      robotList.value = robotList.value.filter((item) => item.id !== id);
      total.value = robotList.value.length;
      message.success('删除成功');
      return { success: true };
    } catch (error) {
      message.error('删除失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // Batch delete
  const batchDeleteRobots = async (ids: number[]) => {
    loading.value = true;
    try {
      await new Promise((resolve) => setTimeout(resolve, 500));

      robotList.value = robotList.value.filter((item) => !ids.includes(item.id));
      total.value = robotList.value.length;
      message.success(`成功删除 ${ids.length} 个机器人`);
      return { success: true };
    } catch (error) {
      message.error('批量删除失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // Start robot
  const startRobot = async (id: number) => {
    try {
      await new Promise((resolve) => setTimeout(resolve, 300));

      const robot = robotList.value.find((item) => item.id === id);
      if (robot) {
        robot.status = 'running';
        robot.lastRunTime = new Date().toLocaleString('zh-CN');
        robot.updateTime = new Date().toLocaleString('zh-CN');
        robot.exitMsg = undefined;
      }
      message.success('启动成功');
      return { success: true };
    } catch (error) {
      message.error('启动失败');
      return { success: false };
    }
  };

  // Stop robot
  const stopRobot = async (id: number) => {
    try {
      await new Promise((resolve) => setTimeout(resolve, 300));

      const robot = robotList.value.find((item) => item.id === id);
      if (robot) {
        robot.status = 'stopped';
        robot.lastStopTime = new Date().toLocaleString('zh-CN');
        robot.updateTime = new Date().toLocaleString('zh-CN');
      }
      message.success('停止成功');
      return { success: true };
    } catch (error) {
      message.error('停止失败');
      return { success: false };
    }
  };

  // Kill robot (force stop)
  const killRobot = async (id: number) => {
    try {
      await new Promise((resolve) => setTimeout(resolve, 500));

      const robot = robotList.value.find((item) => item.id === id);
      if (robot) {
        robot.status = 'stopped';
        robot.lastStopTime = new Date().toLocaleString('zh-CN');
        robot.updateTime = new Date().toLocaleString('zh-CN');
        robot.exitMsg = '强制停止';
      }
      message.success('强制停止成功');
      return { success: true };
    } catch (error) {
      message.error('强制停止失败');
      return { success: false };
    }
  };

  // Pin/Unpin robot
  const togglePinRobot = async (id: number) => {
    try {
      await new Promise((resolve) => setTimeout(resolve, 200));

      const robot = robotList.value.find((item) => item.id === id);
      if (robot) {
        robot.isPinned = !robot.isPinned;
        robot.updateTime = new Date().toLocaleString('zh-CN');
        message.success(robot.isPinned ? '置顶成功' : '取消置顶成功');
      }
      return { success: true };
    } catch (error) {
      message.error('操作失败');
      return { success: false };
    }
  };

  // Copy parameters
  const copyParams = async (request: CopyParamsRequest) => {
    loading.value = true;
    try {
      await new Promise((resolve) => setTimeout(resolve, 500));

      const sourceRobot = robotList.value.find((r) => r.id === request.sourceRobotId);
      if (!sourceRobot) {
        message.error('源机器人不存在');
        return { success: false };
      }

      request.targetRobotIds.forEach((targetId) => {
        const targetRobot = robotList.value.find((r) => r.id === targetId);
        if (targetRobot && targetRobot.strategy === sourceRobot.strategy) {
          targetRobot.strategyParams = { ...sourceRobot.strategyParams };
          targetRobot.updateTime = new Date().toLocaleString('zh-CN');
        }
      });

      message.success('参数复制成功');
      return { success: true };
    } catch (error) {
      message.error('参数复制失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // Batch modify parameters
  const batchModifyParams = async (request: BatchModifyParams) => {
    loading.value = true;
    try {
      await new Promise((resolve) => setTimeout(resolve, 500));

      request.robotIds.forEach((id) => {
        const robot = robotList.value.find((r) => r.id === id);
        if (robot) {
          robot.strategyParams = { ...robot.strategyParams, ...request.params };
          robot.updateTime = new Date().toLocaleString('zh-CN');
        }
      });

      message.success(`成功修改 ${request.robotIds.length} 个机器人的参数`);
      return { success: true };
    } catch (error) {
      message.error('批量修改参数失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // Batch switch version
  const batchSwitchVersion = async (robotIds: number[], version: string) => {
    loading.value = true;
    try {
      await new Promise((resolve) => setTimeout(resolve, 500));

      robotIds.forEach((id) => {
        const robot = robotList.value.find((r) => r.id === id);
        if (robot) {
          robot.strategyVersion = version;
          robot.updateTime = new Date().toLocaleString('zh-CN');
        }
      });

      message.success(`成功切换 ${robotIds.length} 个机器人的版本`);
      return { success: true };
    } catch (error) {
      message.error('批量切换版本失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // Batch add remark
  const batchAddRemark = async (robotIds: number[], remark: string) => {
    loading.value = true;
    try {
      await new Promise((resolve) => setTimeout(resolve, 300));

      robotIds.forEach((id) => {
        const robot = robotList.value.find((r) => r.id === id);
        if (robot) {
          robot.remark = remark;
          robot.updateTime = new Date().toLocaleString('zh-CN');
        }
      });

      message.success(`成功为 ${robotIds.length} 个机器人添加备注`);
      return { success: true };
    } catch (error) {
      message.error('批量添加备注失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // Get status history (mock)
  const getStatusHistory = async (robotId: number): Promise<StatusHistoryEntry[]> => {
    await new Promise((resolve) => setTimeout(resolve, 300));

    // Generate mock history
    const history: StatusHistoryEntry[] = [
      {
        id: 1,
        robotId,
        fromStatus: 'stopped',
        toStatus: 'running',
        message: '手动启动',
        timestamp: '2024-01-31 08:00:00',
        operator: 'admin',
      },
      {
        id: 2,
        robotId,
        fromStatus: 'running',
        toStatus: 'stopped',
        message: '手动停止',
        timestamp: '2024-01-30 18:00:00',
        operator: 'admin',
      },
      {
        id: 3,
        robotId,
        fromStatus: 'stopped',
        toStatus: 'running',
        message: '定时启动',
        timestamp: '2024-01-30 08:00:00',
        operator: 'system',
      },
    ];

    return history;
  };

  // Reset balance
  const resetBalance = async (id: number) => {
    loading.value = true;
    try {
      await new Promise((resolve) => setTimeout(resolve, 500));

      const robot = robotList.value.find((r) => r.id === id);
      if (robot) {
        robot.currentBalance = 10000;
        robot.yield = 0;
        robot.volume = 0;
        robot.equityData = generateEquityData(10000, 1);
        robot.lastResetTime = new Date().toLocaleString('zh-CN');
        robot.updateTime = new Date().toLocaleString('zh-CN');
      }

      message.success('重置权益成功');
      return { success: true };
    } catch (error) {
      message.error('重置权益失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // Apply filters
  const applyFilters = (filters: FilterConditions) => {
    filterConditions.value = filters;
  };

  // Clear filters
  const clearFilters = () => {
    filterConditions.value = {};
  };

  // Get robot by ID
  const getRobotById = (id: number): Robot | undefined => {
    return robotList.value.find((r) => r.id === id);
  };

  return {
    robotList,
    loading,
    total,
    filterConditions,
    getRobotList,
    createRobot,
    updateRobot,
    deleteRobot,
    batchDeleteRobots,
    startRobot,
    stopRobot,
    killRobot,
    togglePinRobot,
    copyParams,
    batchModifyParams,
    batchSwitchVersion,
    batchAddRemark,
    getStatusHistory,
    resetBalance,
    applyFilters,
    clearFilters,
    getRobotById,
  };
});
