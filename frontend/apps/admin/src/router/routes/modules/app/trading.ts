import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';

const routes: RouteRecordRaw[] = [
  {
    path: '/trading',
    name: 'TradingManagement',
    component: BasicLayout,
    redirect: '/trading/actor',
    meta: {
      order: 100,
      icon: 'lucide:bot',
      title: '交易管理',
    },
    children: [
      {
        path: 'actor',
        name: 'ActorManagement',
        meta: {
          icon: 'lucide:cpu',
          title: 'Actor管理',
        },
        component: () => import('#/views/app/trading/actor/index.vue'),
      },
      {
        path: 'robot',
        name: 'RobotManagement',
        meta: {
          icon: 'lucide:bot',
          title: 'Robot管理',
        },
        component: () => import('#/views/app/trading/robot/index.vue'),
      },
      {
        path: 'exchange-account',
        name: 'ExchangeAccount',
        meta: {
          icon: 'lucide:key',
          title: '交易账号',
        },
        component: () => import('#/views/app/trading/exchange-account/index.vue'),
      },
    ],
  },
];

export default routes;
