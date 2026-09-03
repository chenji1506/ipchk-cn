<script setup lang="ts">
const { t } = useI18n()
import { computed } from 'vue';

// 五维雷达：信誉/一致/黑名单/稳定/数据质量
interface DimensionItem {
  score: number;
  max_score: number;
}

const props = defineProps<{
  dimensions: Record<string, DimensionItem>;
  size?: number;
}>();

const size = computed(() => props.size || 260);
const center = computed(() => size.value / 2);
const radius = computed(() => size.value * 0.36);

// 固定轴顺序 + 中文标签
const AXES = [
  { key: 'reputation', label: t('信誉') },
  { key: 'consistency', label: t('一致') },
  { key: 'rbl', label: t('黑名单') },
  { key: 'stability', label: t('稳定') },
  { key: 'data_quality', label: t('数据') },
];

// 每个维度的归一化百分比 0-100
const values = computed(() => {
  return AXES.map((a) => {
    const d = props.dimensions?.[a.key];
    if (!d || d.max_score <= 0) return 0;
    return Math.min(100, Math.max(0, (d.score / d.max_score) * 100));
  });
});

function point(percent: number, index: number): string {
  const angle = (Math.PI * 2 * index) / AXES.length - Math.PI / 2;
  const r = (percent / 100) * radius.value;
  return `${center.value + r * Math.cos(angle)},${center.value + r * Math.sin(angle)}`;
}

function outerPoint(index: number): string {
  return point(100, index);
}

const polygonPoints = computed(() =>
  values.value.map((v, i) => point(v, i)).join(' '),
);
const gridPoints = computed(() =>
  AXES.map((_, i) => outerPoint(i)).join(' '),
);
const labelPositions = computed(() =>
  AXES.map((_, i) => {
    const p = point(112, i).split(',');
    return { x: parseFloat(p[0]), y: parseFloat(p[1]) };
  }),
);
</script>

<template>
  <div class="radar-wrap" :style="{ width: size + 'px', height: size + 'px' }">
    <svg :width="size" :height="size" :viewBox="`0 0 ${size} ${size}`">
      <!-- 网格环（3 层） -->
      <polygon
        v-for="n in 3" :key="n"
        :points="AXES.map((_, i) => point(100 * n / 3, i)).join(' ')"
        fill="none" stroke="#e5e7eb" stroke-width="1"
        :class="{ 'grid-dark': true }"
      />
      <!-- 轴线 -->
      <line
        v-for="(_, i) in AXES" :key="'l' + i"
        :x1="center" :y1="center"
        :x2="outerPoint(i).split(',')[0]" :y2="outerPoint(i).split(',')[1]"
        stroke="#e5e7eb" stroke-width="1" class="grid-dark"
      />
      <!-- 数据多边形 -->
      <polygon :points="polygonPoints" fill="rgba(62,175,124,0.22)" stroke="#3EAF7C" stroke-width="2" stroke-linejoin="round" />
      <!-- 顶点 -->
      <circle
        v-for="(v, i) in values" :key="'d' + i"
        :cx="point(v, i).split(',')[0]" :cy="point(v, i).split(',')[1]"
        r="3.5" fill="#3EAF7C"
      />
      <!-- 标签 -->
      <text
        v-for="(a, i) in AXES" :key="'t' + i"
        :x="labelPositions[i].x" :y="labelPositions[i].y + 4"
        text-anchor="middle" class="axis-label"
      >{{ a.label }}</text>
    </svg>
  </div>
</template>

<style scoped>
.radar-wrap {
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
html.dark .grid-dark {
  stroke: #3a3a3a;
}
.axis-label {
  font-size: 12px;
  fill: #606266;
}
html.dark .axis-label {
  fill: #c0c4cc;
}
</style>
