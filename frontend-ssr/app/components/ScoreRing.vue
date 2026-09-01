<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{
  score: number;      // 0-100，越高越好
  size?: number;      // 直径 px，默认 130
  label?: string;     // 副标题，默认 '/ 100'
}>();

const size = computed(() => props.size || 130);
const stroke = computed(() => Math.round(size.value * 0.085));
const radius = computed(() => (size.value - stroke.value) / 2);
const circumference = computed(() => 2 * Math.PI * radius.value);
const clamped = computed(() => Math.min(100, Math.max(0, props.score)));
const offset = computed(() => circumference.value * (1 - clamped.value / 100));

function ringColor(score: number): string {
  if (score >= 85) return '#3EAF7C';
  if (score >= 65) return '#67C23A';
  if (score >= 40) return '#E6A23C';
  return '#F56C6C';
}
</script>

<template>
  <div class="score-ring" :style="{ width: size + 'px', height: size + 'px' }">
    <svg :width="size" :height="size" :viewBox="`0 0 ${size} ${size}`">
      <circle
        :cx="size / 2" :cy="size / 2" :r="radius"
        fill="none" stroke="#e5e7eb" :stroke-width="stroke"
      />
      <circle
        :cx="size / 2" :cy="size / 2" :r="radius"
        fill="none"
        :stroke="ringColor(clamped)"
        :stroke-width="stroke"
        stroke-linecap="round"
        :stroke-dasharray="circumference"
        :stroke-dashoffset="offset"
        :transform="`rotate(-90 ${size / 2} ${size / 2})`"
        class="ring-progress"
      />
    </svg>
    <div class="ring-center">
      <strong :style="{ color: ringColor(clamped) }">{{ clamped }}</strong>
      <span>{{ label || '/ 100' }}</span>
    </div>
  </div>
</template>

<style scoped>
.score-ring {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
html.dark .score-ring circle[stroke="#e5e7eb"] {
  stroke: #333;
}
.ring-progress {
  transition: stroke-dashoffset 0.8s ease;
}
.ring-center {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
.ring-center strong {
  font-size: 1.9em;
  line-height: 1;
  font-weight: 700;
}
.ring-center span {
  font-size: 0.78em;
  color: #909399;
  margin-top: 4px;
}
</style>
