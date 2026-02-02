import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';

const routes: RouteRecordRaw[] = [
  {
    path: '/trading',
    name: 'TradingManagement',
    component: BasicLayout,
    redirect: '/trading/hft-robots',
    meta: {
      order: 100,
      icon: 'lucide:bot',
      title: '交易管理',
    },
    children: [
      {
        path: 'exchange-account',
        name: 'ExchangeAccount',
        meta: {
          icon: 'lucide:key',
          title: '交易账号',
        },
        component: () => import('#/views/app/trading/exchange-account/index.vue'),
      },
      {
        path: 'server',
        name: 'ServerManagement',
        meta: {
          icon: 'lucide:server',
          title: '托管者管理',
        },
        component: () => import('#/views/app/trading/server/index.vue'),
      },
      {
        path: 'hft-robots',
        name: 'HftRobots',
        meta: {
          icon: 'lucide:bot',
          title: '高频做市',
        },
        component: () => import('#/views/app/hft-robots/index.vue'),
      },
      {
        path: 'strategy',
        name: 'StrategyManagement',
        meta: {
          icon: 'lucide:settings',
          title: '策略管理',
        },
        component: () => import('#/views/app/strategy/index.vue'),
      },
      {
        path: 'asset',
        name: 'AssetManagement',
        meta: {
          icon: 'lucide:wallet',
          title: '我的资产',
        },
        component: () => import('#/views/app/asset/index.vue'),
      },
    ],
  },
];

export default routes;
