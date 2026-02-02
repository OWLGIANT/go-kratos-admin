import { defineStore } from 'pinia';
import { ref } from 'vue';
import { message } from 'ant-design-vue';

export interface StrategyParam {
  id: number;
  description: string;
  defaultValue: string;
}

export interface Strategy {
  id: number;
  strategyName: string;
  strategyNickname: string;
  strategyParams: StrategyParam[];
  isUsingHub: boolean;
  createTime: string;
}

export const useStrategyStore = defineStore('strategy', () => {
  const strategyList = ref<Strategy[]>([]);
  const loading = ref(false);

  // 模拟获取策略列表
  const getStrategyList = async () => {
    loading.value = true;
    try {
      // 模拟API调用，返回默认数据
      await new Promise(resolve => setTimeout(resolve, 500));

      strategyList.value = [
        {
          id: 1,
          strategyName: 'HFT_MARKET_MAKING',
          strategyNickname: '高频做市策略',
          isUsingHub: true,
          createTime: '2024-01-15 10:30:00',
          strategyParams: [
            { id: 1, description: '最小价差', defaultValue: '0.001' },
            { id: 2, description: '订单数量', defaultValue: '100' },
            { id: 3, description: '最大持仓', defaultValue: '10000' },
            { id: 4, description: '风险系数', defaultValue: '0.5' },
          ],
        },
        {
          id: 2,
          strategyName: 'GRID_TRADING',
          strategyNickname: '网格交易策略',
          isUsingHub: false,
          createTime: '2024-01-20 14:20:00',
          strategyParams: [
            { id: 5, description: '网格数量', defaultValue: '10' },
            { id: 6, description: '网格间距', defaultValue: '0.5%' },
            { id: 7, description: '初始资金', defaultValue: '10000' },
          ],
        },
        {
          id: 3,
          strategyName: 'ARBITRAGE',
          strategyNickname: '套利策略',
          isUsingHub: true,
          createTime: '2024-01-25 09:15:00',
          strategyParams: [
            { id: 8, description: '最小套利空间', defaultValue: '0.2%' },
            { id: 9, description: '交易对', defaultValue: 'BTC/USDT' },
            { id: 10, description: '最大滑点', defaultValue: '0.1%' },
          ],
        },
      ];

      return { success: true, data: { list: strategyList.value } };
    } catch (error) {
      message.error('获取策略列表失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // 模拟删除策略
  const deleteStrategy = async (id: number) => {
    loading.value = true;
    try {
      // 模拟API调用
      await new Promise(resolve => setTimeout(resolve, 300));

      strategyList.value = strategyList.value.filter(item => item.id !== id);
      message.success('删除成功');
      return { success: true };
    } catch (error) {
      message.error('删除失败');
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // 模拟同步策略
  const syncStrategy = async () => {
    try {
      // 模拟API调用
      await new Promise(resolve => setTimeout(resolve, 500));

      message.success('同步成功');
      await getStrategyList();
      return { msg: 'ok' };
    } catch (error) {
      message.error('同步失败');
      return { msg: 'error' };
    }
  };

  return {
    strategyList,
    loading,
    getStrategyList,
    deleteStrategy,
    syncStrategy,
  };
});
