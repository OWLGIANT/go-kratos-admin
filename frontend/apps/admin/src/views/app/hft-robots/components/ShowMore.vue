<template>
  <div class="show-more">
    <span v-if="!expanded" class="content-preview">
      {{ preview }}
      <a v-if="isTruncated" @click="toggleExpand" class="expand-link">展开</a>
    </span>
    <span v-else class="content-full">
      {{ content }}
      <a @click="toggleExpand" class="expand-link">收起</a>
    </span>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';

interface Props {
  content: string;
  maxLength?: number;
}

const props = withDefaults(defineProps<Props>(), {
  maxLength: 50,
});

const expanded = ref(false);

const isTruncated = computed(() => props.content.length > props.maxLength);

const preview = computed(() => {
  if (!isTruncated.value) return props.content;
  return props.content.substring(0, props.maxLength) + '...';
});

const toggleExpand = () => {
  expanded.value = !expanded.value;
};
</script>

<style scoped>
.show-more {
  display: inline-block;
  max-width: 100%;
  word-break: break-all;
}

.content-preview,
.content-full {
  white-space: pre-wrap;
  word-break: break-word;
}

.expand-link {
  margin-left: 4px;
  color: #1890ff;
  cursor: pointer;
  user-select: none;
}

.expand-link:hover {
  color: #40a9ff;
}
</style>
