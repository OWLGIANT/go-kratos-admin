/**
 * HFT Robots Types
 * TypeScript interfaces for high-frequency trading robots management
 */

// Robot status enum
export type RobotStatus = 'running' | 'stopped' | 'error' | 'starting' | 'stopping';

// Exchange types
export type ExchangeType = 'okx_spot' | 'bybit_spot' | 'binance_spot' | 'gate_spot';

// Strategy parameter types
export type ParamType = 'float64' | 'int' | 'string' | 'bool' | 'selected';

// Equity data point for charts
export interface EquityDataPoint {
  time: string;
  equity: number;
  timestamp: number;
}

// Strategy parameter definition
export interface StrategyParam {
  name: string;
  type: ParamType;
  description: string;
  tips: string;
  default: any;
  options?: string[]; // For 'selected' type
  min?: number;
  max?: number;
}

// Strategy definition
export interface Strategy {
  id: string;
  name: string;
  description: string;
  params: StrategyParam[];
  versions: string[];
}

// Robot interface (extended)
export interface Robot {
  id: number;
  nickname: string;
  account: string;
  exchange: ExchangeType;
  exchangeName: string;
  pair: string;
  strategy: string;
  strategyVersion: string;
  strategyParams: Record<string, any>;
  status: RobotStatus;

  // Financial data
  currentBalance: number;
  yield: number;
  volume: number;
  cash: number;
  coin: number;

  // Performance metrics
  equityData: EquityDataPoint[];
  makerFee: number;
  takerFee: number;

  // Timing data
  createTime: string;
  updateTime: string;
  lastResetTime?: string;
  lastRunTime?: string;
  lastStopTime?: string;

  // Delays (in milliseconds)
  newDelay?: number;
  cancelDelay?: number;
  systemDelay?: number;

  // Server and error info
  server?: string;
  exitMsg?: string;

  // Metadata
  creator: string;
  remark?: string;
  tags?: string[];
  isPinned: boolean;
}

// Robot form data (for create/edit)
export interface RobotFormData {
  id?: number;
  nickname: string;
  account: string;
  exchange: ExchangeType;
  pair: string;
  strategy: string;
  strategyVersion: string;
  strategyParams: Record<string, any>;
  remark?: string;
}

// Filter conditions
export interface FilterConditions {
  exchange?: ExchangeType | '';
  pair?: string;
  status?: RobotStatus | '';
  strategy?: string;
  creator?: string;
  keyword?: string;
}

// Column configuration
export interface ColumnConfig {
  key: string;
  label: string;
  selected: boolean;
  width?: number;
  fixed?: 'left' | 'right';
  sortable?: boolean;
}

// Batch operation types
export type BatchOperationType = 'modify' | 'delete' | 'switchVersion' | 'addRemark' | 'start' | 'stop';

// Batch modify params
export interface BatchModifyParams {
  robotIds: number[];
  params: Record<string, any>;
}

// Copy params request
export interface CopyParamsRequest {
  sourceRobotId: number;
  targetRobotIds: number[];
}

// Status history entry
export interface StatusHistoryEntry {
  id: number;
  robotId: number;
  fromStatus: RobotStatus;
  toStatus: RobotStatus;
  message?: string;
  timestamp: string;
  operator?: string;
}

// Export options
export interface ExportOptions {
  robotIds?: number[];
  format: 'csv' | 'json' | 'excel';
  includeEquityData: boolean;
  includeParams: boolean;
}

// Account info
export interface Account {
  id: string;
  name: string;
  exchange: ExchangeType;
  apiKey: string;
  balance: number;
}

// Trading pair info
export interface TradingPair {
  symbol: string;
  baseAsset: string;
  quoteAsset: string;
  exchange: ExchangeType;
  minQty: number;
  maxQty: number;
  tickSize: number;
}
