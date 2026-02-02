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
