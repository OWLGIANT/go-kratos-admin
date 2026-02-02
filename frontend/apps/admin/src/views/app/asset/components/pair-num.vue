<template>
  <div class="pair-num-container">
    <div class="time-picker">
      <a-radio-group v-model:value="duration" size="small" @change="handleTimeChange">
        <a-radio-button value="24h">24h</a-radio-button>
        <a-radio-button value="7d">7d</a-radio-button>
        <a-radio-button value="15d">15d</a-radio-button>
        <a-radio-button value="30d">30d</a-radio-button>
      </a-radio-group>
    </div>
    <div class="chart-wrapper">
      <v-chart :option="chartOption" :loading="assetStore.loading" style="height: 420px" autoresize />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { use } from 'echarts/core';
import { CanvasRenderer } from 'echarts/renderers';
import { LineChart } from 'echarts/charts';
import {
  TitleComponent,
  TooltipComponent,
  GridComponent,
  DataZoomComponent,
  LegendComponent,
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
  LegendComponent,
]);

const assetStore = useAssetStore();
const duration = ref('24h');

const chartOption = computed(() => {
  const xData = assetStore.pairNumData.time?.map((t) =>
    dayjs(t).format('MM-DD HH:mm:ss')
  );
  const yData = assetStore.pairNumData.count?.map((item) => item?.toFixed(2));

  return {
    title: {
      text: 'GOAL币种数量统计',
      left: 'center',
      top: 'top',
    },
    tooltip: {
      trigger: 'axis',
    },
    grid: {
      left: '10%',
      right: '10%',
      containLabel: true,
    },
    xAxis: {
      type: 'category',
      boundaryGap: false,
      data: xData,
      axisLine: { show: true },
      axisTick: { show: true },
      axisLabel: { show: true, margin: 10 },
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
        type: 'line',
        data: yData,
        showSymbol: false,
        itemStyle: { color: '#52c41a' },
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
  };
});

const handleTimeChange = () => {
  let hours = 24;
  if (duration.value.includes('h')) {
    hours = Number(duration.value.replace('h', ''));
  } else if (duration.value.includes('d')) {
    hours = Number(duration.value.replace('d', '')) * 24;
  }
  assetStore.getPairNumData(hours);
};

onMounted(() => {
  assetStore.getPairNumData(24);
});
</script>

<style scoped lang="scss">
.pair-num-container {
  .time-picker {
    margin-bottom: 16px;
  }

  .chart-wrapper {
    background: #fff;
    padding: 16px;
    border-radius: 4px;
  }
}
</style>
