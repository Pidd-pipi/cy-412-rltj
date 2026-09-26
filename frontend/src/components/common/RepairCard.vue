<template>
  <article class="card repair-card">
    <div class="repair-main">
      <h3>{{ repair.title }}</h3>
      <p>{{ repair.description }}</p>
      <small>
        {{ repair.type }} · {{ repair.user?.nickname || '业主' }} · {{ new Date(repair.created_at).toLocaleString() }}
        <template v-if="repair.handler"> · 处理人：{{ repair.handler.nickname }}</template>
      </small>

      <div v-if="repair.rating > 0" class="repair-rating">
        <el-rate :model-value="repair.rating" disabled />
        <el-tag v-if="repair.status === 'closed'" type="success" size="small">验收通过</el-tag>
        <el-tag v-else type="danger" size="small">已退回返工</el-tag>
      </div>
      <div v-if="repair.rework_reason" class="repair-rework">
        返工原因：{{ repair.rework_reason }}
      </div>

      <div v-if="canAccept" class="repair-accept">
        <el-divider content-position="left">完工验收</el-divider>
        <el-rate v-model="rating" :texts="RATE_TEXTS" show-text />
        <el-input
          v-if="rating > 0 && rating <= 3"
          v-model="reworkReason"
          type="textarea"
          :rows="2"
          maxlength="200"
          show-word-limit
          placeholder="请填写返工原因（1-3 分必填），工单将退回原处理人继续处理"
          class="repair-rework-input"
        />
        <div class="repair-actions">
          <el-button
            type="primary"
            :loading="submitting"
            :disabled="rating === 0 || (rating <= 3 && !reworkReason.trim())"
            @click="submitAccept"
          >{{ rating >= 4 ? '确认验收并关单' : '退回返工' }}</el-button>
        </div>
      </div>

      <div v-else-if="canStaff" class="repair-actions">
        <el-button
          v-if="repair.status === 'assigned'"
          size="small"
          :loading="submitting"
          @click="submitStatus('processing')"
        >开始处理</el-button>
        <el-button
          v-if="repair.status === 'processing'"
          size="small"
          type="primary"
          :loading="submitting"
          @click="submitStatus('acceptance')"
        >提交完工</el-button>
      </div>
    </div>
    <RepairStatusBadge :status="repair.status" />
  </article>
</template>
<script setup lang="ts">
import {computed, ref} from 'vue';
import {ElMessage} from 'element-plus';
import type {PropType} from 'vue';
import type {Repair} from '../../types';
import {useAuth} from '../../hooks/useAuth';
import {acceptRepair, updateRepairStatus} from '../../api/repair';
import RepairStatusBadge from './RepairStatusBadge.vue';

const props = defineProps({repair: {type: Object as PropType<Repair>, required: true}});
const emit = defineEmits<{(e:'changed'):void}>();

const {user} = useAuth();
const RATE_TEXTS = ['', '很差', '较差', '一般', '满意', '非常满意'];
const rating = ref(0);
const reworkReason = ref('');
const submitting = ref(false);

const isOwner = computed(() => user.value?.id === props.repair.user_id);
const canAccept = computed(
  () => isOwner.value && (props.repair.status === 'acceptance' || props.repair.status === 'done'),
);
const canStaff = computed(
  () => ['staff', 'admin'].includes(user.value?.role || '') &&
    ['assigned', 'processing'].includes(props.repair.status),
);

async function submitAccept() {
  if (rating.value === 0) return;
  if (rating.value <= 3 && !reworkReason.value.trim()) {
    ElMessage.warning('评 1-3 分时请填写返工原因');
    return;
  }
  submitting.value = true;
  try {
    await acceptRepair(props.repair.id, rating.value, reworkReason.value);
    ElMessage.success(rating.value >= 4 ? '验收通过，工单已关闭' : '已填写返工原因，工单退回处理人');
    emit('changed');
  } catch (e) {
    ElMessage.error((e as Error).message);
  } finally {
    submitting.value = false;
  }
}

async function submitStatus(status: 'processing' | 'acceptance') {
  submitting.value = true;
  try {
    await updateRepairStatus(props.repair.id, status);
    ElMessage.success(status === 'acceptance' ? '已提交完工，等待业主验收' : '已开始处理');
    emit('changed');
  } catch (e) {
    ElMessage.error((e as Error).message);
  } finally {
    submitting.value = false;
  }
}
</script>
<style scoped>
.repair-card { align-items: flex-start; }
.repair-main { min-width: 0; }
.repair-rating { display: flex; align-items: center; gap: 10px; margin-top: 10px; }
.repair-rework {
  margin-top: 8px; padding: 8px 12px; max-width: 560px;
  background: #fef0f0; color: #c45656; border-radius: 8px; font-size: 13px;
}
.repair-accept { margin-top: 6px; max-width: 560px; }
.repair-rework-input { margin-top: 10px; }
.repair-actions { margin-top: 12px; display: flex; gap: 8px; }
</style>
