<template>
  <div class="asset-page">
    <a-page-header title="我的资产" />

    <div class="tabs-container">
      <a-tabs v-model:activeKey="selectedTab" type="card" @change="handleTabChange">
        <a-tab-pane key="pair_num" tab="GOAL币种数量统计">
          <pair-num v-if="selectedTab === 'pair_num'" />
        </a-tab-pane>
        <a-tab-pane key="position_currency" tab="各牧场数据统计">
          <pair-distribution v-if="selectedTab === 'position_currency'" :selected-tab="selectedTab" />
        </a-tab-pane>
        <a-tab-pane key="total_fund" tab="总数据">
          <total-fund v-if="selectedTab === 'total_fund'" />
        </a-tab-pane>
      </a-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import PairNum from './components/pair-num.vue';
import PairDistribution from './components/pair-distribution.vue';
import TotalFund from './components/total-fund.vue';

const route = useRoute();
const router = useRouter();
const selectedTab = ref('pair_num');

const handleTabChange = (key: string) => {
  selectedTab.value = key;
  router.push({ query: { tab: key } });
};

onMounted(() => {
  const tab = (route.query.tab as string) || 'pair_num';
  selectedTab.value = tab;
});

watch(
  () => route.query.tab,
  (newTab) => {
    if (newTab && typeof newTab === 'string') {
      selectedTab.value = newTab;
    }
  }
);
</script>

<style scoped lang="scss">
.asset-page {
  padding: 16px;

  .tabs-container {
    margin-top: 16px;
    background: #fff;
    padding: 16px;
    border-radius: 4px;
  }
}
</style>
