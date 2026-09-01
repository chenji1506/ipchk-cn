<script setup lang="ts">
import { computed } from 'vue';

// 场景风险评估：基于 IP 画像的启发式星级（参考 ipchk.cc 设计）
interface Scenario {
  name: string;
  stars: number;
  label: string;
  tone: 'good' | 'warn' | 'bad';
  icon: string;
  reason: string;
}

const props = defineProps<{
  report: any;
}>();

const flags = computed(() => {
  const r = props.report || {};
  const primary = r.profile?.primary || '';
  const ipType = r.ip_type || '';
  const isDatacenter = primary === '机房IP' || ipType === '机房IP';
  const isResidential = primary === '住宅IP' || primary === '移动网络IP' || ipType === '住宅IP';
  const hasRbl = (r.rbl?.listed_count || 0) > 0 || (r.rbl?.network_listed_count || 0) > 0;
  const hasProxy = (r.purity?.signal_risk || 0) >= 20;
  return { isDatacenter, isResidential, hasRbl, hasProxy };
});

const scenarios = computed<Scenario[]>(() => {
  const f = flags.value;
  const severe = f.hasRbl || f.hasProxy;

  function ev(name: string, base: number, goodReason: string, riskReason: string): Scenario {
    let stars = base;
    let reason = goodReason;
    if (severe) {
      stars -= 1;
      reason = f.hasRbl ? '存在黑名单记录，需要先复核' : '代理或 VPN 属性会提高平台风控';
    }
    stars = Math.max(1, Math.min(5, stars));
    const label = stars >= 5 ? '极佳' : stars === 4 ? '良好' : stars === 3 ? '谨慎使用' : stars === 2 ? '风险较高' : '不建议';
    const tone = stars >= 4 ? 'good' : stars === 3 ? 'warn' : 'bad';
    const icon = stars >= 4 ? 'ri-checkbox-circle-fill' : stars === 3 ? 'ri-error-warning-fill' : 'ri-close-circle-fill';
    return { name, stars, label, tone, icon, reason: severe ? reason : (stars >= 4 ? goodReason : riskReason) };
  }

  return [
    ev('TikTok', (f.isResidential ? 1 : 0) + (f.isDatacenter ? -1 : 0) + 3,
      '住宅或移动网络更接近普通用户环境',
      f.isDatacenter ? '机房IP更容易触发账号与地区风控' : '画像证据不足，建议小号先测试'),
    ev('跨境电商', (f.isResidential ? 1 : 0) + (f.isDatacenter ? -1 : 0) + 3,
      '住宅网络且公开信誉良好',
      f.isDatacenter ? '机房IP不适合注册、支付等敏感操作' : '注册和支付前建议人工复核'),
    ev('社媒运营', (f.isResidential ? 1 : 0) + (f.isDatacenter ? -1 : 0) + 3,
      '用户型网络更适合日常登录和运营',
      f.isDatacenter ? '机房出口可能影响登录稳定性' : '建议先进行低频登录测试'),
    ev('AI 应用', (f.isDatacenter ? 1 : 0) + 3,
      '机房 IP 适合 API、服务器和自建应用',
      '服务可用性仍受地区和平台策略影响'),
  ];
});
</script>

<template>
  <div class="scenario-risk-grid">
    <article v-for="s in scenarios" :key="s.name" class="scenario-card" :class="'tone-' + s.tone">
      <div class="scenario-head">
        <span class="scenario-name">{{ s.name }}</span>
        <span class="scenario-stars" :title="`${s.stars} 星`">
          <span v-for="n in 5" :key="n" class="star" :class="{ active: n <= s.stars }">★</span>
        </span>
      </div>
      <strong class="scenario-label">{{ s.label }}</strong>
      <small class="scenario-reason">{{ s.reason }}</small>
    </article>
  </div>
</template>

<style scoped>
.scenario-risk-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 14px;
}
.scenario-card {
  border: 1px solid #e4e7ed;
  border-radius: 10px;
  padding: 14px 16px;
  background: #fafbfc;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
html.dark .scenario-card {
  border-color: #333;
  background: #1a1a1a;
}
.scenario-card.tone-good { border-left: 4px solid #3EAF7C; }
.scenario-card.tone-warn { border-left: 4px solid #E6A23C; }
.scenario-card.tone-bad { border-left: 4px solid #F56C6C; }
.scenario-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.scenario-name {
  font-weight: 600;
  font-size: 1.02em;
}
.scenario-stars {
  letter-spacing: 1px;
}
.star {
  color: #dcdfe6;
  font-size: 0.95em;
}
.star.active {
  color: #f7ba2a;
}
.scenario-label {
  font-size: 1.05em;
}
.scenario-card.tone-good .scenario-label { color: #3EAF7C; }
.scenario-card.tone-warn .scenario-label { color: #E6A23C; }
.scenario-card.tone-bad .scenario-label { color: #F56C6C; }
.scenario-reason {
  color: #909399;
  font-size: 0.85em;
  line-height: 1.5;
}
</style>
