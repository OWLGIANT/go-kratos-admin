/**
 * HFT Robots Configuration
 * Column definitions, mock strategies, and configuration management
 */

import type { ColumnConfig, Strategy, ExchangeType } from './types';

// LocalStorage keys
export const STORAGE_KEY = {
  COLUMNS: 'HFT_ROBOTS_COLUMNS',
  COLUMN_WIDTHS: 'HFT_ROBOTS_COLUMN_WIDTHS',
  FILTERS: 'HFT_ROBOTS_FILTERS',
};

// Default column configuration
export const DEFAULT_COLUMNS: ColumnConfig[] = [
  { key: 'id', label: 'ID', selected: true, width: 80, fixed: 'left' },
  { key: 'nickname', label: '机器人名称', selected: true, width: 180 },
  { key: 'account', label: '账号', selected: true, width: 120 },
  { key: 'yield', label: '增长率', selected: true, width: 100, sortable: true },
  { key: 'currentBalance', label: '当前权益', selected: true, width: 120, sortable: true },
  { key: 'equity', label: '权益曲线', selected: true, width: 180 },
  { key: 'status', label: '机器人状态', selected: true, width: 110 },
  { key: 'pair', label: '交易对', selected: true, width: 120 },
  { key: 'strategy', label: '策略', selected: true, width: 150 },
  { key: 'strategyVersion', label: '策略版本', selected: true, width: 120 },
  { key: 'strategyParams', label: '机器人参数', selected: false, width: 250 },
  { key: 'volume', label: '成交金额', selected: true, width: 120, sortable: true },
  { key: 'cash', label: 'Cash', selected: false, width: 100 },
  { key: 'coin', label: 'Coin', selected: false, width: 100 },
  { key: 'server', label: '交易服务器', selected: false, width: 150 },
  { key: 'exitMsg', label: '停机(错误)日志', selected: false, width: 200 },
  { key: 'exchangeName', label: '平台', selected: true, width: 120 },
  { key: 'creator', label: '创建人', selected: false, width: 100 },
  { key: 'remark', label: '备注', selected: true, width: 150 },
  { key: 'createTime', label: '创建时间', selected: true, width: 180 },
  { key: 'lastResetTime', label: '上次复位时间', selected: false, width: 180 },
  { key: 'lastRunTime', label: '上次启动时间', selected: false, width: 180 },
  { key: 'lastStopTime', label: '上次停机时间', selected: false, width: 180 },
  { key: 'newDelay', label: '下单延迟(ms)', selected: false, width: 120 },
  { key: 'cancelDelay', label: '撤单延迟(ms)', selected: false, width: 120 },
  { key: 'systemDelay', label: '系统延迟(ms)', selected: false, width: 120 },
  { key: 'makerFee', label: 'Maker手续费', selected: false, width: 120 },
  { key: 'takerFee', label: 'Taker手续费', selected: false, width: 120 },
  { key: 'operation', label: '操作', selected: true, width: 400, fixed: 'right' },
];

// Mock strategy definitions
export const MOCK_STRATEGIES: Strategy[] = [
  {
    id: 'match',
    name: '做市策略',
    description: '经典做市商策略，通过买卖价差获利',
    versions: ['v1.0.0', 'v1.1.0', 'v1.2.0', 'v2.0.0'],
    params: [
      {
        name: 'spread',
        type: 'float64',
        description: '价差',
        tips: '买卖价差百分比，例如0.002表示0.2%',
        default: 0.002,
        min: 0.0001,
        max: 0.1,
      },
      {
        name: 'volume',
        type: 'float64',
        description: '单次交易量',
        tips: '每次下单的数量',
        default: 100,
        min: 1,
        max: 100000,
      },
      {
        name: 'max_position',
        type: 'float64',
        description: '最大持仓',
        tips: '最大持仓限制，超过此值将停止开仓',
        default: 10000,
        min: 0,
        max: 1000000,
      },
      {
        name: 'order_interval',
        type: 'int',
        description: '下单间隔(ms)',
        tips: '两次下单之间的时间间隔',
        default: 1000,
        min: 100,
        max: 60000,
      },
      {
        name: 'enable_hedge',
        type: 'bool',
        description: '启用对冲',
        tips: '是否启用对冲功能，对冲可以降低风险',
        default: true,
      },
      {
        name: 'price_offset',
        type: 'float64',
        description: '价格偏移',
        tips: '相对中间价的偏移量',
        default: 0,
        min: -0.01,
        max: 0.01,
      },
    ],
  },
  {
    id: 'grid',
    name: '网格策略',
    description: '在价格区间内设置多个网格，低买高卖',
    versions: ['v1.0.0', 'v1.1.0', 'v2.0.0'],
    params: [
      {
        name: 'grid_num',
        type: 'int',
        description: '网格数量',
        tips: '网格的数量，数量越多，网格越密集',
        default: 10,
        min: 2,
        max: 100,
      },
      {
        name: 'price_range',
        type: 'float64',
        description: '价格区间(%)',
        tips: '网格覆盖的价格区间百分比',
        default: 5.0,
        min: 0.1,
        max: 50,
      },
      {
        name: 'order_amount',
        type: 'float64',
        description: '每格金额',
        tips: '每个网格的下单金额',
        default: 100,
        min: 10,
        max: 100000,
      },
      {
        name: 'profit_ratio',
        type: 'float64',
        description: '止盈比例',
        tips: '止盈的比例，达到此比例后平仓',
        default: 0.01,
        min: 0.001,
        max: 0.5,
      },
      {
        name: 'stop_loss_ratio',
        type: 'float64',
        description: '止损比例',
        tips: '止损的比例，达到此比例后停止策略',
        default: 0.05,
        min: 0.01,
        max: 0.5,
      },
      {
        name: 'grid_mode',
        type: 'selected',
        description: '网格模式',
        tips: '选择网格的排列模式',
        default: 'arithmetic',
        options: ['arithmetic', 'geometric', 'fibonacci'],
      },
    ],
  },
  {
    id: 'twap',
    name: 'TWAP策略',
    description: '时间加权平均价格策略，在指定时间内均匀执行订单',
    versions: ['v1.0.0', 'v1.1.0'],
    params: [
      {
        name: 'total_amount',
        type: 'float64',
        description: '总交易量',
        tips: '需要完成的总交易量',
        default: 1000,
        min: 1,
        max: 1000000,
      },
      {
        name: 'duration',
        type: 'int',
        description: '执行时长(分钟)',
        tips: '完成交易的时间长度',
        default: 60,
        min: 1,
        max: 1440,
      },
      {
        name: 'slice_num',
        type: 'int',
        description: '切片数量',
        tips: '将总量分成多少份执行',
        default: 20,
        min: 2,
        max: 1000,
      },
      {
        name: 'price_limit',
        type: 'float64',
        description: '价格限制(%)',
        tips: '相对市场价的偏离限制',
        default: 0.5,
        min: 0.1,
        max: 10,
      },
      {
        name: 'randomize',
        type: 'bool',
        description: '随机化执行',
        tips: '是否随机化每次执行的时间和数量',
        default: false,
      },
      {
        name: 'side',
        type: 'selected',
        description: '交易方向',
        tips: '买入或卖出',
        default: 'buy',
        options: ['buy', 'sell'],
      },
    ],
  },
  {
    id: 'dual_match',
    name: '双边做市策略',
    description: '同时在买卖两边挂单，保持库存平衡',
    versions: ['v1.0.0', 'v1.1.0', 'v1.2.0'],
    params: [
      {
        name: 'bid_spread',
        type: 'float64',
        description: '买单价差',
        tips: '买单相对中间价的价差',
        default: 0.001,
        min: 0.0001,
        max: 0.1,
      },
      {
        name: 'ask_spread',
        type: 'float64',
        description: '卖单价差',
        tips: '卖单相对中间价的价差',
        default: 0.001,
        min: 0.0001,
        max: 0.1,
      },
      {
        name: 'order_size',
        type: 'float64',
        description: '订单大小',
        tips: '每个订单的大小',
        default: 50,
        min: 1,
        max: 100000,
      },
      {
        name: 'refresh_interval',
        type: 'int',
        description: '刷新间隔(ms)',
        tips: '订单刷新的时间间隔',
        default: 500,
        min: 100,
        max: 60000,
      },
      {
        name: 'inventory_target',
        type: 'float64',
        description: '目标库存',
        tips: '目标持仓数量，策略会尝试维持此库存',
        default: 0,
        min: -100000,
        max: 100000,
      },
      {
        name: 'max_inventory',
        type: 'float64',
        description: '最大库存',
        tips: '最大持仓限制',
        default: 5000,
        min: 0,
        max: 1000000,
      },
      {
        name: 'skew_enabled',
        type: 'bool',
        description: '启用倾斜',
        tips: '根据库存情况调整买卖价差',
        default: true,
      },
    ],
  },
];

// Exchange options
export const EXCHANGE_OPTIONS: { value: ExchangeType; label: string }[] = [
  { value: 'okx_spot', label: 'OKX现货' },
  { value: 'bybit_spot', label: 'Bybit现货' },
  { value: 'binance_spot', label: 'Binance现货' },
  { value: 'gate_spot', label: 'Gate现货' },
];

// Status options
export const STATUS_OPTIONS = [
  { value: 'running', label: '运行中', color: 'success' },
  { value: 'stopped', label: '已停止', color: 'default' },
  { value: 'error', label: '错误', color: 'error' },
  { value: 'starting', label: '启动中', color: 'processing' },
  { value: 'stopping', label: '停止中', color: 'warning' },
];

// Helper functions
export function loadColumnConfig(): ColumnConfig[] {
  try {
    const saved = localStorage.getItem(STORAGE_KEY.COLUMNS);
    if (saved) {
      const savedColumns = JSON.parse(saved);
      // Merge with defaults to handle new columns
      return DEFAULT_COLUMNS.map((defaultCol) => {
        const savedCol = savedColumns.find((c: ColumnConfig) => c.key === defaultCol.key);
        return savedCol ? { ...defaultCol, ...savedCol } : defaultCol;
      });
    }
  } catch (error) {
    console.error('Failed to load column config:', error);
  }
  return DEFAULT_COLUMNS;
}

export function saveColumnConfig(columns: ColumnConfig[]): void {
  try {
    localStorage.setItem(STORAGE_KEY.COLUMNS, JSON.stringify(columns));
  } catch (error) {
    console.error('Failed to save column config:', error);
  }
}

export function getStrategyById(strategyId: string): Strategy | undefined {
  return MOCK_STRATEGIES.find((s) => s.id === strategyId);
}

export function getStrategyDefaultParams(strategyId: string): Record<string, any> {
  const strategy = getStrategyById(strategyId);
  if (!strategy) return {};

  const params: Record<string, any> = {};
  strategy.params.forEach((param) => {
    params[param.name] = param.default;
  });
  return params;
}

export function getExchangeLabel(exchange: ExchangeType): string {
  const option = EXCHANGE_OPTIONS.find((opt) => opt.value === exchange);
  return option?.label || exchange;
}

export function getStatusConfig(status: string) {
  return STATUS_OPTIONS.find((opt) => opt.value === status) || STATUS_OPTIONS[1];
}
