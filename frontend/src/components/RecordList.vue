<script setup>
defineProps({
  records: {
    type: Array,
    default: () => [],
  },
});

defineEmits(["undo"]);

function formatScore(value) {
  const number = Number(value || 0);
  return number > 0 ? `+${number}` : String(number);
}

function formatDate(value) {
  if (!value) return "";
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}
</script>

<template>
  <div class="record-list">
    <div v-if="!records.length" class="empty compact">暂无积分记录</div>
    <article v-for="record in records" :key="record.id" class="record-item" :class="{ undone: !record.effective }">
      <div>
        <div class="record-main">
          <strong>{{ record.studentName }}</strong>
          <span>{{ record.reason }}</span>
        </div>
        <p>
          {{ record.ruleName }} · {{ formatDate(record.createdAt) }}
          <template v-if="record.note"> · {{ record.note }}</template>
        </p>
        <p v-if="!record.effective">已撤销：{{ record.undoReason || "未填写原因" }}</p>
      </div>
      <div class="record-side">
        <em :class="{ negative: record.score < 0 }">{{ formatScore(record.score) }}</em>
        <button v-if="record.effective" class="ghost small" type="button" @click="$emit('undo', record)">撤销</button>
      </div>
    </article>
  </div>
</template>
