<template>
  <a-dropdown>
    <a-button>
      <template #icon>
        <ExportOutlined />
      </template>
      导出
    </a-button>
    <template #overlay>
      <a-menu @click="handleExport">
        <a-menu-item key="csv">
          <FileTextOutlined />
          导出为 CSV
        </a-menu-item>
        <a-menu-item key="json">
          <FileOutlined />
          导出为 JSON
        </a-menu-item>
        <a-menu-item key="excel">
          <FileExcelOutlined />
          导出为 Excel
        </a-menu-item>
      </a-menu>
    </template>
  </a-dropdown>
</template>

<script setup lang="ts">
import { message } from 'ant-design-vue';
import {
  ExportOutlined,
  FileTextOutlined,
  FileOutlined,
  FileExcelOutlined,
} from '@ant-design/icons-vue';
import { useRobotStore } from '#/stores';

interface Props {
  selectedIds?: number[];
}

const props = defineProps<Props>();

const robotStore = useRobotStore();

const handleExport = ({ key }: { key: string }) => {
  const format = key as 'csv' | 'json' | 'excel';

  // Get robots to export
  const robotsToExport =
    props.selectedIds && props.selectedIds.length > 0
      ? robotStore.robotList.filter((r) => props.selectedIds!.includes(r.id))
      : robotStore.robotList;

  if (robotsToExport.length === 0) {
    message.warning('没有可导出的数据');
    return;
  }

  try {
    if (format === 'csv') {
      exportAsCSV(robotsToExport);
    } else if (format === 'json') {
      exportAsJSON(robotsToExport);
    } else if (format === 'excel') {
      // For Excel, we'll use CSV format as a simple implementation
      exportAsCSV(robotsToExport);
      message.info('Excel格式导出功能开发中，已导出为CSV格式');
    }

    message.success(`成功导出 ${robotsToExport.length} 条记录`);
  } catch (error) {
    console.error('Export failed:', error);
    message.error('导出失败');
  }
};

const exportAsCSV = (robots: any[]) => {
  // CSV headers
  const headers = [
    'ID',
    '机器人名称',
    '账号',
    '交易所',
    '交易对',
    '策略',
    '策略版本',
    '状态',
    '当前权益',
    '增长率(%)',
    '成交金额',
    '创建时间',
    '备注',
  ];

  // CSV rows
  const rows = robots.map((robot) => [
    robot.id,
    robot.nickname,
    robot.account,
    robot.exchangeName,
    robot.pair,
    robot.strategy,
    robot.strategyVersion,
    robot.status,
    robot.currentBalance,
    robot.yield,
    robot.volume,
    robot.createTime,
    robot.remark || '',
  ]);

  // Combine headers and rows
  const csvContent = [headers, ...rows]
    .map((row) => row.map((cell) => `"${cell}"`).join(','))
    .join('\n');

  // Add BOM for UTF-8 encoding (for Excel compatibility)
  const BOM = '\uFEFF';
  const blob = new Blob([BOM + csvContent], { type: 'text/csv;charset=utf-8;' });

  // Download
  const link = document.createElement('a');
  const url = URL.createObjectURL(blob);
  link.setAttribute('href', url);
  link.setAttribute('download', `robots_export_${Date.now()}.csv`);
  link.style.visibility = 'hidden';
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
};

const exportAsJSON = (robots: any[]) => {
  // Create simplified export data
  const exportData = robots.map((robot) => ({
    id: robot.id,
    nickname: robot.nickname,
    account: robot.account,
    exchange: robot.exchangeName,
    pair: robot.pair,
    strategy: robot.strategy,
    strategyVersion: robot.strategyVersion,
    strategyParams: robot.strategyParams,
    status: robot.status,
    currentBalance: robot.currentBalance,
    yield: robot.yield,
    volume: robot.volume,
    createTime: robot.createTime,
    remark: robot.remark,
  }));

  const jsonContent = JSON.stringify(exportData, null, 2);
  const blob = new Blob([jsonContent], { type: 'application/json' });

  // Download
  const link = document.createElement('a');
  const url = URL.createObjectURL(blob);
  link.setAttribute('href', url);
  link.setAttribute('download', `robots_export_${Date.now()}.json`);
  link.style.visibility = 'hidden';
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
};
</script>
