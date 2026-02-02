<template>
  <div class="equity-chart-wrapper">
    <v-chart
      v-if="hasData"
      :option="chartOption"
      :style="{ height: '60px', width: '150px' }"
      autoresize
    />
    <div v-else class="no-data">
      <span style="color: #999; font-size: 12px">暂无数据</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import VChart from 'vue-echarts';
import { use } from 'echarts/core';
import { LineChart } from 'echarts/charts';
import { GridComponent, TooltipComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import type { EquityDataPoint } from '../types';

use([LineChart, GridComponent, TooltipComponent, CanvasRenderer]);

interface Props {
  data: EquityDataPoint[];
  currentBalance: number;
}

const props = defineProps<Props>();

const hasData = computed(() => props.data && props.data.length > 0);

const chartOption = computed(() => {
  if (!hasData.value) return {};

  const equityValues = props.data.map((d) => d.equity);
  const minEquity = Math.min(...equityValues);
  const maxEquity = Math.max(...equityValues);
  const range = maxEquity - minEquity;
  const padding = range * 0.1;

  // Calculate color based on overall trend
  const firstEquity = props.data[0].equity;
  const lastEquity = props.data[props.data.length - 1].equity;
  const lineColor = lastEquity >= firstEquity ? '#52c41a' : '#ff4d4f';

  return {
    grid: {
      top: 5,
      left: 5,
      right: 5,
      bottom: 5,
      containLabel: false,
    },
    xAxis: {
      type: 'category',
      show: false,
      data: props.data.map((d) => d.time),
      boundaryGap: false,
    },
    yAxis: {
      type: 'value',
      show: false,
      min: minEquity - padding,
      max: maxEquity + padding,
    },
    series: [
      {
        name: '权益',
        data: equityValues,
        type: 'line',
        showSymbol: false,
        smooth: true,
        lineStyle: {
          width: 1.5,
          color: lineColor,
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              {
                offset: 0,
                color: lineColor + '40',
              },
              {
                offset: 1,
                color: lineColor + '10',
              },
            ],
          },
        },
      },
      {
        name: '+1%参考线',
        data: props.data.map(() => props.currentBalance * 1.01),
        type: 'line',
        showSymbol: false,
        lineStyle: {
          type: 'dashed',
          color: '#d9d9d9',
          width: 1,
        },
        silent: true,
      },
      {
        name: '-1%参考线',
        data: props.data.map(() => props.currentBalance * 0.99),
        type: 'line',
        showSymbol: false,
        lineStyle: {
          type: 'dashed',
          color: '#d9d9d9',
          width: 1,
        },
        silent: true,
      },
    ],
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(0, 0, 0, 0.8)',
      borderColor: 'transparent',
      textStyle: {
        color: '#fff',
        fontSize: 12,
      },
      formatter: (params: any) => {
        if (!params || params.length === 0) return '';
        const dataPoint = params[0];
        return `${dataPoint.name}<br/>权益: ${dataPoint.data.toFixed(2)}`;
      },
    },
  };
});
</script>

<style scoped>
.equity-chart-wrapper {
  display: inline-block;
  vertical-align: middle;
}

.no-data {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 60px;
  width: 150px;
}
</style>
