import { defineStore } from 'pinia';
import { ref } from 'vue';
import dayjs from 'dayjs';

export interface TimeSeriesData {
  time: string[];
  count?: number[];
  equity?: number[];
}

export interface ExchangeData {
  [exchange: string]: TimeSeriesData;
}

export const useAssetStore = defineStore('asset', () => {
  const pairNumData = ref<TimeSeriesData>({ time: [], count: [] });
  const totalFundData = ref<TimeSeriesData>({ time: [], count: [] });
  const exchangeData = ref<ExchangeData>({});
  const loading = ref(false);

  // 生成模拟时间序列数据
  const generateTimeSeriesData = (hours: number, baseValue: number, variance: number) => {
    const now = dayjs();
    const time: string[] = [];
    const values: number[] = [];

    for (let i = hours; i >= 0; i--) {
      time.push(now.subtract(i, 'hour').format('YYYY-MM-DD HH:mm:ss'));
      const randomChange = (Math.random() - 0.5) * variance;
      values.push(baseValue + randomChange);
    }

    return { time, values };
  };

  // 获取币种数量统计
  const getPairNumData = async (duration: number = 24) => {
    loading.value = true;
    try {
      await new Promise(resolve => setTimeout(resolve, 500));

      const { time, values } = generateTimeSeriesData(duration, 150, 30);
      pairNumData.value = {
        time,
        count: values,
      };

      return { success: true, data: pairNumData.value };
    } catch (error) {
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // 获取总资金数据
  const getTotalFundData = async (duration: number = 24) => {
    loading.value = true;
    try {
      await new Promise(resolve => setTimeout(resolve, 500));

      const { time, values } = generateTimeSeriesData(duration, 1000000, 50000);
      totalFundData.value = {
        time,
        count: values,
      };

      return { success: true, data: totalFundData.value };
    } catch (error) {
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  // 获取各交易所资金分布
  const getExchangeFundDistribution = async (exchange?: string, duration: number = 24) => {
    loading.value = true;
    try {
      await new Promise(resolve => setTimeout(resolve, 500));

      const exchanges = exchange
        ? [exchange]
        : [
            'okx_spot',
            'okx_usdt_swap',
            'bybit_spot',
            'bybit_usdt_swap',
            'gate_spot',
            'gate_usdt_swap',
            'bitget_spot',
            'bitget_usdt_swap',
          ];

      const newData: ExchangeData = {};

      exchanges.forEach((ex) => {
        const { time, values } = generateTimeSeriesData(duration, 100000 + Math.random() * 50000, 10000);
        newData[ex] = {
          time,
          equity: values,
        };
      });

      if (exchange) {
        exchangeData.value = {
          ...exchangeData.value,
          ...newData,
        };
      } else {
        exchangeData.value = newData;
      }

      return { success: true, data: exchangeData.value };
    } catch (error) {
      return { success: false };
    } finally {
      loading.value = false;
    }
  };

  return {
    pairNumData,
    totalFundData,
    exchangeData,
    loading,
    getPairNumData,
    getTotalFundData,
    getExchangeFundDistribution,
  };
});
