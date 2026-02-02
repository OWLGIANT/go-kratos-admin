<template>
  <div class="pair-distribution-container">
    <a-spin :spinning="!selectedExchange && assetStore.loading">
      <div class="charts-grid">
        <div
          v-for="exchange in filteredExchanges"
          :key="exchange"
          class="chart-item"
        >
          <div class="time-picker">
            <a-radio-group
              :value="duration"
              size="small"
              @change="(e) => handleTimeChange(e, exchange)"
            >
              <a-radio-button value="24h">24h</a-radio-button>
              <a-radio-button value="7d">7d</a-radio-button>
              <a-radio-button value="15d">15d</a-radio-button>
              <a-radio-button value="30d">30d</a-radio-button>
            </a-radio-group>
          </div>
          <v-chart
            :option="getChartOption(exchange)"
            :loading="selectedExchange === exchange && assetStore.loading"
            style="height: 420px"
            autoresize
          />
        </div>
      </div>
    </a-spin>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue';
import { use } from 'echarts/core';
import { CanvasRenderer } from 'echarts/renderers';
import { LineChart } from 'echarts/charts';
import {
  TitleComponent,
  TooltipComponent,
  GridComponent,
  DataZoomComponent,
} from 'echarts/components';
import VChart from 'vue-echarts';
import dayjs from 'dayjs';
import { useAssetStore } from '#/stores';

use([
  CanvasRenderer,
  LineChart,
  TitleComponent,
  TooltipComponent,
  GridComponent,
  DataZoomComponent,
]);

interface Props {
  selectedTab: string;
}

const props = defineProps<Props>();
const assetStore = useAssetStore();
const duration = ref('24h');
const selectedExchange = ref('');

const exchangeFilter = [
  'okx_spot',
  'okx_usdt_swap',
  'bybit_spot',
  'bybit_usdt_swap',
  'gate_spot',
  'gate_usdt_swap',
  'bitget_spot',
  'bitget_usdt_swap',
];

const filteredExchanges = computed(() => {
  return Object.keys(assetStore.exchangeData).filter((exchange) => {
    const data = assetStore.exchangeData[exchange];
    const hasData = data?.equity?.some((item) => item !== 0);
    return hasData && exchangeFilter.includes(exchange);
  });
});

const getChartOption = (exchange: string) => {
  const data = assetStore.exchangeData[exchange];
  const xData = data?.time?.map((t) => dayjs(t).format('MM-DD HH:mm:ss')) || [];
  const yData = data?.equity?.map((item) => item?.toFixed(2)) || [];

  return {
    title: {
      text: `${exchange}数据情况`,
      left: 'center',
    },
    grid: {
      left: '19%',
      right: '19%',
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: xData,
      axisLine: { show: true },
      axisTick: {
        show: true,
        interval: (index: number) => index === 0 || index === xData.length - 1,
      },
      axisLabel: {
        show: true,
        margin: 10,
        interval: 0,
        formatter: (value: string, index: number) => {
          if (index === 0 || index === xData.length - 1) {
            return value;
          }
          return '';
        },
      },
    },
    yAxis: {
      type: 'value',
      scale: true,
      axisLine: { show: true },
      axisTick: { show: true },
      splitLine: { show: true },
    },
    dataZoom: [
      {
        show: true,
        realtime: true,
        start: 0,
        end: 100,
      },
      {
        type: 'inside',
        realtime: true,
        start: 30,
        end: 70,
      },
    ],
    series: [
      {
        name: '金额',
        type: 'line',
        data: yData,
        showSymbol: false,
        itemStyle: { color: '#91CC75' },
        lineStyle: { color: '#4693FF', width: 1 },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: '#CCDCFE' },
              { offset: 1, color: '#F8FAFE' },
            ],
          },
        },
      },
    ],
    tooltip: {
      trigger: 'axis',
    },
  };
};

const handleTimeChange = (e: any, exchange: string) => {
  let hours = 24;
  const value = e.target.value;
  if (value.includes('h')) {
    hours = Number(value.replace('h', ''));
  } else if (value.includes('d')) {
    hours = Number(value.replace('d', '')) * 24;
  }
  duration.value = value;
  selectedExchange.value = exchange;
  assetStore.getExchangeFundDistribution(exchange, hours);
};

watch(
  () => props.selectedTab,
  (newTab) => {
    if (newTab !== 'position_currency') {
      selectedExchange.value = '';
      duration.value = '24h';
    }
  }
);

onMounted(() => {
  if (props.selectedTab === 'position_currency') {
    assetStore.getExchangeFundDistribution(undefined, 24);
  }
});
</script>

<style scoped lang="scss">
.pair-distribution-container {
  .charts-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(600px, 1fr));
    gap: 24px;

    .chart-item {
      background: #fff;
      padding: 16px;
      border-radius: 4px;

      .time-picker {
        margin-bottom: 16px;
      }
    }
  }
}
</style>
