<script setup>
import { computed, onMounted, reactive, ref } from "vue";
import {
  Award,
  BarChart3,
  BookOpen,
  Check,
  ChevronLeft,
  CirclePlus,
  ClipboardList,
  Download,
  Home,
  LogIn,
  Pencil,
  Plus,
  RotateCcw,
  Save,
  Search,
  Settings,
  Sparkles,
  Trash2,
  Trophy,
  Upload,
  Users,
  UsersRound,
  X,
} from "lucide-vue-next";
import { apiDelete, apiGet, apiPost, apiPut } from "./api";
import { createXlsxBlob } from "./xlsx";
import RecordList from "./components/RecordList.vue";
import readXlsxFile from "read-excel-file/browser";

const pages = [
  { key: "home", label: "班级首页", icon: Home },
  { key: "classroom", label: "课堂加分", icon: Sparkles },
  { key: "students", label: "学生管理", icon: Users },
  { key: "groups", label: "小组管理", icon: UsersRound },
  { key: "rules", label: "积分规则", icon: ClipboardList },
  { key: "records", label: "积分记录", icon: BookOpen },
  { key: "ranking", label: "排行榜", icon: Trophy },
  { key: "settings", label: "系统设置", icon: Settings },
];


const user = ref(localStorage.getItem("classpoints-user") || "");
const activePage = ref("classes");
const selectedClassId = ref(Number(localStorage.getItem("classpoints-class-id") || 0));
const loading = ref(false);
const message = ref("");
const messageTimer = ref(null);

const classes = ref([]);
const dashboard = ref(null);
const students = ref([]);
const studentGroups = ref([]);
const rules = ref([]);
const records = ref([]);
const ranking = ref([]);
const settings = ref(null);

const classModal = ref(false);
const studentModal = ref(false);
const editingStudentId = ref(null);
const importModal = ref(false);
const groupModal = ref(false);
const editingGroupId = ref(null);
const ruleModal = ref(false);
const batchModal = ref(false);
const undoModal = ref(false);
const undoRecordTarget = ref(null);
const undoReason = ref("误操作");
const undoSubmitting = ref(false);

const loginForm = reactive({ name: "老师" });
const classForm = reactive({ name: "", teacher: "" });
const studentForm = reactive({ name: "", code: "", groupName: "", gender: "" });
const importForm = reactive({ text: "" });
const groupForm = reactive({ name: "" });
const ruleForm = reactive({ name: "", score: 1, category: "课堂表现" });
const batchForm = reactive({ studentIds: [], ruleId: "", score: 1, reason: "", note: "" });
const settingForm = reactive({ schoolName: "", backupDir: "", autoBackupMins: 30, backupKeepCount: 20 });

const studentKeyword = ref("");
const recordFilter = ref("all");
const recordStudentId = ref("");
const recordStartDate = ref("");
const recordEndDate = ref("");
const rankScope = ref("all");
const rankStartDate = ref("");
const rankEndDate = ref("");
const draggingStudentId = ref(null);
const dragTargetGroup = ref("");
const suppressStudentClick = ref(false);

const rankScopes = [
  { value: "all", label: "全部" },
  { value: "today", label: "今日" },
  { value: "week", label: "本周" },
  { value: "month", label: "本月" },
  { value: "last7", label: "最近一周" },
  { value: "lastMonth", label: "最近一个月" },
];
const studentCodeCollator = new Intl.Collator("zh-CN", { numeric: true, sensitivity: "base" });

function compareStudentsByCode(left, right) {
  const leftCode = String(left.code || "").trim();
  const rightCode = String(right.code || "").trim();
  if (!leftCode && rightCode) return 1;
  if (leftCode && !rightCode) return -1;

  const codeResult = studentCodeCollator.compare(leftCode, rightCode);
  if (codeResult !== 0) return codeResult;

  const nameResult = studentCodeCollator.compare(String(left.name || ""), String(right.name || ""));
  return nameResult || Number(left.id) - Number(right.id);
}

const selectedClass = computed(() => safeArray(classes.value).find((item) => item.id === selectedClassId.value));
const enabledRules = computed(() => safeArray(rules.value).filter((rule) => rule.enabled));
const filteredStudents = computed(() => {
  const keyword = studentKeyword.value.trim().toLowerCase();
  if (!keyword) return safeArray(students.value);
  return safeArray(students.value).filter((student) => {
    return `${student.name} ${student.code} ${student.gender} ${student.groupName}`.toLowerCase().includes(keyword);
  });
});
const topThree = computed(() => safeArray(ranking.value).slice(0, 3));
const classroomStudents = computed(() => filteredStudents.value);
const classroomGroups = computed(() => {
  const groups = new Map();
  studentGroups.value.forEach((group) => groups.set(group.name, []));
  groups.set("未分组", []);
  classroomStudents.value.forEach((student, index) => {
    const name = String(student.groupName || "").trim() || "未分组";
    if (!groups.has(name)) groups.set(name, []);
    groups.get(name).push({ student, index });
  });
  return [...groups.entries()].map(([name, items]) => ({
    name,
    items: items.sort((left, right) => compareStudentsByCode(left.student, right.student)),
  }));
});
const selectedClassroomStudents = computed(() => {
  const selectedIds = new Set(batchForm.studentIds.map(String));
  return students.value.filter((student) => selectedIds.has(String(student.id)));
});
const batchSelectedCount = computed(() => {
  const studentIds = new Set(safeArray(students.value).map((student) => String(student.id)));
  return batchForm.studentIds.filter((id) => studentIds.has(String(id))).length;
});
const importRows = computed(() => buildImportRows(importForm.text));
const importReadyRows = computed(() => importRows.value.filter((row) => row.valid && !row.duplicate));
const importSummary = computed(() => {
  const rows = importRows.value;
  return {
    total: rows.length,
    ready: importReadyRows.value.length,
    invalid: rows.filter((row) => !row.valid).length,
    duplicate: rows.filter((row) => row.valid && row.duplicate).length,
  };
});

onMounted(async () => {
  if (!user.value) {
    activePage.value = "login";
    return;
  }
  await loadClasses();
  if (selectedClassId.value) {
    activePage.value = "home";
    await loadClassData();
  } else {
    activePage.value = "classes";
  }
});

async function login() {
  try {
    const result = await apiPost("/login", loginForm);
    user.value = result.name;
    localStorage.setItem("classpoints-user", result.name);
    activePage.value = "classes";
    await loadClasses();
  } catch (error) {
    toast(error.message);
  }
}

function logout() {
  user.value = "";
  selectedClassId.value = 0;
  localStorage.removeItem("classpoints-user");
  localStorage.removeItem("classpoints-class-id");
  activePage.value = "login";
}

async function loadClasses() {
  await run(async () => {
    classes.value = safeArray(await apiGet("/classes"));
    if (selectedClassId.value && !classes.value.some((item) => item.id === selectedClassId.value)) {
      selectedClassId.value = 0;
      localStorage.removeItem("classpoints-class-id");
    }
  });
}

async function createClass() {
  if (!classForm.name.trim()) return toast("请输入班级名称");
  try {
    const created = await apiPost("/classes", classForm);
    classes.value.unshift(created);
    classModal.value = false;
    classForm.name = "";
    classForm.teacher = "";
    await selectClass(created.id);
    toast("班级已创建");
  } catch (error) {
    toast(error.message);
  }
}

async function deleteClass(item) {
  const confirmed = window.confirm(`确定删除班级“${item.name}”吗？\n\n该班级的所有学生和积分记录将被永久删除，分组和积分规则会保留。`);
  if (!confirmed) return;
  try {
    await apiDelete(`/classes/${item.id}`);
    classes.value = classes.value.filter((classItem) => classItem.id !== item.id);
    if (selectedClassId.value === item.id) {
      selectedClassId.value = 0;
      localStorage.removeItem("classpoints-class-id");
    }
    toast("班级已删除");
  } catch (error) {
    toast(error.message);
  }
}

async function selectClass(id) {
  selectedClassId.value = id;
  localStorage.setItem("classpoints-class-id", String(id));
  activePage.value = "home";
  await loadClassData();
}

async function loadClassData() {
  if (!selectedClassId.value) return;
  await run(async () => {
    const id = selectedClassId.value;
    const [dash, stu, groupList, ruleList, recordList, rank] = await Promise.all([
      apiGet(`/classes/${id}/dashboard`),
      apiGet(`/classes/${id}/students`),
      apiGet(`/classes/${id}/groups`),
      apiGet(`/classes/${id}/rules`),
      apiGet(`/classes/${id}/records?${buildRecordQuery()}`),
      apiGet(`/classes/${id}/ranking?${buildRankingQuery()}`),
    ]);
    dashboard.value = {
      ...dash,
      recentRecords: safeArray(dash?.recentRecords),
      ranking: safeArray(dash?.ranking),
    };
    students.value = safeArray(stu);
    studentGroups.value = safeArray(groupList);
    rules.value = safeArray(ruleList);
    records.value = safeArray(recordList);
    ranking.value = safeArray(rank);
    ensureScoreDefaults();
  });
}

async function loadRecords() {
  if (!selectedClassId.value) return;
  records.value = safeArray(await apiGet(`/classes/${selectedClassId.value}/records?${buildRecordQuery()}`));
}

function buildRecordQuery() {
  const params = new URLSearchParams({ filter: recordFilter.value });
  if (recordStudentId.value) params.set("studentId", recordStudentId.value);
  if (recordStartDate.value) params.set("startDate", recordStartDate.value);
  if (recordEndDate.value) params.set("endDate", recordEndDate.value);
  return params.toString();
}

function clearRecordFilters() {
  recordFilter.value = "all";
  recordStudentId.value = "";
  recordStartDate.value = "";
  recordEndDate.value = "";
}

async function resetRecordFilters() {
  clearRecordFilters();
  await loadRecords();
}

function exportRecordsExcel() {
  if (!records.value.length) return toast("当前筛选条件下没有可导出的积分记录");

  const headers = ["序号", "学生姓名", "性别", "积分规则", "分值", "原因", "备注", "记录状态", "撤销原因", "记录时间"];
  const rows = records.value.map((record, index) => [
    index + 1,
    record.studentName,
    record.gender || "",
    record.ruleName || "自定义",
    Number(record.score || 0),
    record.reason,
    record.note || "",
    record.effective ? "有效" : "已撤销",
    record.undoReason || "",
    formatExportDate(record.createdAt),
  ]);
  const blob = createXlsxBlob("积分记录", [headers, ...rows]);
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  const className = sanitizeFileName(selectedClass.value?.name || "班级");
  link.href = url;
  link.download = `${className}-积分记录-${localDateText(new Date())}.xlsx`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
  toast(`已导出 ${records.value.length} 条积分记录`);
}

function sanitizeFileName(value) {
  return String(value || "班级").replace(/[\\/:*?"<>|]/g, "-").trim() || "班级";
}

function localDateText(date) {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}${month}${day}`;
}

function formatExportDate(value) {
  if (!value) return "";
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(new Date(value));
}

async function loadRanking() {
  if (!selectedClassId.value) return;
  ranking.value = safeArray(await apiGet(`/classes/${selectedClassId.value}/ranking?${buildRankingQuery()}`));
}

function buildRankingQuery() {
  const params = new URLSearchParams({ scope: rankScope.value });
  if (rankScope.value === "custom") {
    if (rankStartDate.value) params.set("startDate", rankStartDate.value);
    if (rankEndDate.value) params.set("endDate", rankEndDate.value);
  }
  return params.toString();
}

async function selectRankScope(scope) {
  rankScope.value = scope;
  await loadRanking();
}

async function applyCustomRankingDates() {
  if (rankStartDate.value && rankEndDate.value && rankStartDate.value > rankEndDate.value) {
    return toast("开始日期不能晚于结束日期");
  }
  rankScope.value = "custom";
  await loadRanking();
}

function ensureScoreDefaults() {
  if (!batchForm.ruleId && enabledRules.value.length) applyRuleToForm(batchForm, enabledRules.value[0].id);
}

function applyRuleToForm(form, ruleId) {
  form.ruleId = String(ruleId);
  const rule = rules.value.find((item) => item.id === Number(ruleId));
  if (!rule) return;
  form.score = rule.score;
  form.reason = rule.name;
}

function openCreateStudent() {
  editingStudentId.value = null;
  Object.assign(studentForm, { name: "", code: "", groupName: "", gender: "" });
  studentModal.value = true;
}

function openEditStudent(student) {
  editingStudentId.value = student.id;
  Object.assign(studentForm, {
    name: student.name || "",
    code: student.code || "",
    groupName: student.groupName || "",
    gender: student.gender || "",
  });
  studentModal.value = true;
}

function closeStudentModal() {
  studentModal.value = false;
  editingStudentId.value = null;
  Object.assign(studentForm, { name: "", code: "", groupName: "", gender: "" });
}

function openImportStudents() {
  importForm.text = "";
  importModal.value = true;
}

function closeImportStudents() {
  importModal.value = false;
  importForm.text = "";
}

async function saveStudent() {
  if (!studentForm.name.trim()) return toast("请输入学生姓名");
  try {
    if (editingStudentId.value) {
      await apiPut(`/students/${editingStudentId.value}`, studentForm);
      toast("学生信息已更新");
    } else {
      await apiPost(`/classes/${selectedClassId.value}/students`, studentForm);
      toast("学生已添加");
    }
    closeStudentModal();
    await loadClassData();
  } catch (error) {
    toast(error.message);
  }
}

async function handleImportFile(event) {
  const file = event.target.files?.[0];
  if (!file) return;
  try {
    if (isXlsxFile(file)) {
      const rows = await readXlsxFile(file);
      importForm.text = rows.map((row) => row.map(formatImportCell).join("\t")).join("\n");
    } else {
      importForm.text = await file.text();
    }
  } catch {
    toast("文件读取失败，请确认文件格式或复制表格内容后粘贴导入");
  } finally {
    event.target.value = "";
  }
}

async function importStudents() {
  if (!importReadyRows.value.length) return toast("没有可导入的学生");
  let success = 0;
  let failed = 0;
  for (const row of importReadyRows.value) {
    try {
      await apiPost(`/classes/${selectedClassId.value}/students`, {
        name: row.name,
        code: row.code,
        groupName: row.groupName,
        gender: row.gender,
      });
      success += 1;
    } catch {
      failed += 1;
    }
  }
  closeImportStudents();
  await loadClassData();
  toast(failed ? `已导入 ${success} 人，失败 ${failed} 人` : `已导入 ${success} 名学生`);
}

async function deleteStudent(student) {
  if (!window.confirm(`删除学生「${student.name}」及其记录？`)) return;
  try {
    await apiDelete(`/students/${student.id}`);
    await loadClassData();
    toast("学生已删除");
  } catch (error) {
    toast(error.message);
  }
}

function openCreateGroup() {
  editingGroupId.value = null;
  groupForm.name = "";
  groupModal.value = true;
}

function openEditGroup(group) {
  editingGroupId.value = group.id;
  groupForm.name = group.name;
  groupModal.value = true;
}

function closeGroupModal() {
  groupModal.value = false;
  editingGroupId.value = null;
  groupForm.name = "";
}

async function saveStudentGroup() {
  const name = groupForm.name.trim();
  if (!name) return toast("请输入小组名称");
  try {
    if (editingGroupId.value) {
      await apiPut(`/groups/${editingGroupId.value}`, { name });
      toast("小组名称已更新");
    } else {
      await apiPost(`/classes/${selectedClassId.value}/groups`, { name });
      toast("小组已创建");
    }
    closeGroupModal();
    await loadClassData();
  } catch (error) {
    toast(error.message);
  }
}

async function deleteStudentGroup(group) {
  const memberText = group.studentCount ? `\n\n组内 ${group.studentCount} 名学生将转为“未分组”，学生和积分记录不会删除。` : "";
  if (!window.confirm(`确定删除小组“${group.name}”吗？${memberText}`)) return;
  try {
    await apiDelete(`/groups/${group.id}`);
    await loadClassData();
    toast("小组已删除");
  } catch (error) {
    toast(error.message);
  }
}

async function createRule() {
  if (!ruleForm.name.trim() || Number(ruleForm.score) === 0) return toast("请输入规则名称和非 0 分值");
  try {
    await apiPost(`/classes/${selectedClassId.value}/rules`, { ...ruleForm, score: Number(ruleForm.score) });
    ruleModal.value = false;
    Object.assign(ruleForm, { name: "", score: 1, category: "课堂表现" });
    await loadClassData();
    toast("规则已添加");
  } catch (error) {
    toast(error.message);
  }
}

async function toggleRule(rule) {
  try {
    await apiPut(`/rules/${rule.id}`, { ...rule, enabled: !rule.enabled });
    await loadClassData();
  } catch (error) {
    toast(error.message);
  }
}

async function deleteRule(rule) {
  if (!window.confirm(`删除规则「${rule.name}」？历史记录会保留规则名称。`)) return;
  try {
    await apiDelete(`/rules/${rule.id}`);
    await loadClassData();
    toast("规则已删除");
  } catch (error) {
    toast(error.message);
  }
}

function handleStudentCardClick(student) {
  if (suppressStudentClick.value) return;
  const studentID = String(student.id);
  if (batchForm.studentIds.includes(studentID)) {
    batchForm.studentIds = batchForm.studentIds.filter((id) => id !== studentID);
  } else {
    batchForm.studentIds = [...batchForm.studentIds, studentID];
  }
}

function startStudentDrag(event, student) {
  draggingStudentId.value = student.id;
  suppressStudentClick.value = true;
  event.dataTransfer.effectAllowed = "move";
  event.dataTransfer.setData("text/plain", String(student.id));
}

function finishStudentDrag() {
  draggingStudentId.value = null;
  dragTargetGroup.value = "";
  window.setTimeout(() => {
    suppressStudentClick.value = false;
  }, 0);
}

async function moveStudentToGroup(studentID, groupName) {
  const student = students.value.find((item) => item.id === Number(studentID));
  if (!student) return;
  const nextGroupName = groupName === "未分组" ? "" : groupName;
  if ((student.groupName || "") === nextGroupName) return;

  try {
    await apiPut(`/students/${student.id}`, {
      name: student.name,
      code: student.code,
      gender: student.gender,
      groupName: nextGroupName,
    });
    await loadClassData();
    toast(`已将「${student.name}」移动到${nextGroupName ? `「${nextGroupName}」` : "未分组"}`);
  } catch (error) {
    toast(error.message);
  }
}

async function dropStudentOnGroup(event, groupName) {
  const studentID = Number(event.dataTransfer.getData("text/plain") || draggingStudentId.value);
  dragTargetGroup.value = "";
  if (!studentID) return;
  await moveStudentToGroup(studentID, groupName);
}

function openBatchScore(selectAll = false) {
  if (selectAll) batchForm.studentIds = students.value.map((student) => String(student.id));
  if (!batchSelectedCount.value) return toast("请先选择学生");
  if (!batchForm.ruleId && enabledRules.value.length) applyRuleToForm(batchForm, enabledRules.value[0].id);
  batchModal.value = true;
}

function closeBatchScore() {
  batchModal.value = false;
}

function clearClassroomSelection() {
  batchForm.studentIds = [];
}

async function createBatchRecords() {
  const validStudentIds = new Set(students.value.map((student) => Number(student.id)));
  const studentIds = [...new Set(batchForm.studentIds.map(Number))].filter((id) => validStudentIds.has(id));
  if (!studentIds.length) return toast("请选择学生");
  if (!batchForm.reason.trim()) return toast("请输入原因");
  try {
    await apiPost(`/classes/${selectedClassId.value}/records/batch`, {
      studentIds,
      ruleId: batchForm.ruleId ? Number(batchForm.ruleId) : null,
      score: Number(batchForm.score),
      reason: batchForm.reason,
      note: batchForm.note,
    });
    batchModal.value = false;
    Object.assign(batchForm, { studentIds: [], ruleId: batchForm.ruleId, score: batchForm.score, reason: batchForm.reason, note: "" });
    clearRecordFilters();
    await loadClassData();
    toast(`已为 ${studentIds.length} 名学生批量加分`);
  } catch (error) {
    toast(error.message);
  }
}

function undoRecord(record) {
  undoRecordTarget.value = record;
  undoReason.value = "误操作";
  undoModal.value = true;
}

function closeUndoModal() {
  if (undoSubmitting.value) return;
  undoModal.value = false;
  undoRecordTarget.value = null;
  undoReason.value = "误操作";
}

async function confirmUndoRecord() {
  const record = undoRecordTarget.value;
  const reason = undoReason.value.trim();
  if (!record) return;
  if (!reason) return toast("请输入撤销原因");

  undoSubmitting.value = true;
  try {
    await apiPost(`/records/${record.id}/undo`, { reason });
    undoModal.value = false;
    undoRecordTarget.value = null;
    await loadClassData();
    toast("记录已撤销");
  } catch (error) {
    toast(error.message);
  } finally {
    undoSubmitting.value = false;
  }
}

async function loadSettings() {
  try {
    const data = await apiGet("/settings");
    settings.value = data;
    Object.assign(settingForm, data);
  } catch (error) {
    toast(error.message);
  }
}

async function saveSettings() {
  try {
    settings.value = await apiPut("/settings", {
      ...settingForm,
      autoBackupMins: Number(settingForm.autoBackupMins),
      backupKeepCount: Number(settingForm.backupKeepCount),
    });
    toast("设置已保存");
  } catch (error) {
    toast(error.message);
  }
}

async function backupNow() {
  try {
    const result = await apiPost("/backup", {});
    toast(`备份完成：${result.path}`);
  } catch (error) {
    toast(error.message);
  }
}

async function goPage(page) {
  activePage.value = page;
  if (page === "settings") await loadSettings();
  if (page === "records") {
    clearRecordFilters();
    await loadRecords();
  }
  if (page === "ranking") await loadRanking();
}

async function run(fn) {
  loading.value = true;
  try {
    await fn();
  } catch (error) {
    toast(error.message);
  } finally {
    loading.value = false;
  }
}

function toast(text) {
  message.value = text;
  clearTimeout(messageTimer.value);
  messageTimer.value = window.setTimeout(() => {
    message.value = "";
  }, 2600);
}

function formatScore(value) {
  const number = Number(value || 0);
  return number > 0 ? `+${number}` : String(number);
}

function formatTotalScore(value) {
  return `${Number(value || 0)}分`;
}

function studentNumber(student, index) {
  const code = String(student.code || "").trim();
  return code || String(index + 1);
}

function avatarClass(student, index) {
  const source = `${student.name || ""}${student.gender || ""}${index}`;
  let hash = 0;
  for (const char of source) hash += char.charCodeAt(0);
  return [`variant-${hash % 6}`, student.gender === "女" ? "girl" : "boy"];
}

function buildImportRows(text) {
  const parsed = parseStudentImportText(text);
  const existingKeys = new Set();
  for (const student of students.value) {
    existingKeys.add(studentImportKey(student.code, student.name, student.gender));
  }

  const seenKeys = new Set();
  return parsed.map((row) => {
    const key = studentImportKey(row.code, row.name, row.gender);
    const valid = Boolean(row.name);
    const duplicate = valid && (existingKeys.has(key) || seenKeys.has(key));
    if (valid) seenKeys.add(key);
    return {
      ...row,
      valid,
      duplicate,
      status: !valid ? "缺少姓名" : duplicate ? "重复，跳过" : "可导入",
    };
  });
}

function parseStudentImportText(text) {
  const lines = String(text || "")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  if (!lines.length) return [];

  const table = lines.map(splitImportLine);
  const headerIndex = table.findIndex((cells) => cells.some(isStudentNameHeader) || cells.some(isCodeHeader));
  let codeIndex = 0;
  let nameIndex = 1;
  let genderIndex = 2;
  let groupIndex = 3;
  let startIndex = 0;

  if (headerIndex >= 0) {
    const header = table[headerIndex];
    codeIndex = findHeaderIndex(header, isCodeHeader, 0);
    nameIndex = findHeaderIndex(header, isStudentNameHeader, 1);
    genderIndex = findHeaderIndex(header, isGenderHeader, 2);
    groupIndex = findHeaderIndex(header, isGroupHeader, 3);
    startIndex = headerIndex + 1;
  }

  const rows = [];
  for (let index = startIndex; index < table.length; index += 1) {
    const cells = table[index];
    const code = cleanImportCell(cells[codeIndex] || "");
    const name = cleanImportCell(cells[nameIndex] || "");
    const gender = normalizeImportGender(cells[genderIndex] || "");
    const groupName = cleanImportCell(cells[groupIndex] || "");
    if (!code && !name && !gender && !groupName) continue;
    if (cells.length === 1 && !name && !gender && !groupName) continue;
    rows.push({ line: index + 1, code, name, gender, groupName });
  }
  return rows;
}

function splitImportLine(line) {
  const normalized = line.replace(/\uFEFF/g, "");
  if (normalized.includes("\t")) return normalized.split("\t").map(cleanImportCell);
  if (normalized.includes(",")) return normalized.split(",").map(cleanImportCell);
  if (normalized.includes("，")) return normalized.split("，").map(cleanImportCell);
  return normalized.split(/\s{2,}/).map(cleanImportCell);
}

function cleanImportCell(value) {
  return String(value || "").trim().replace(/^["']|["']$/g, "");
}

function formatImportCell(value) {
  if (value === null || value === undefined) return "";
  if (value instanceof Date) return "";
  return String(value).trim();
}

function isXlsxFile(file) {
  return /\.xlsx$/i.test(file.name || "") || file.type === "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet";
}

function findHeaderIndex(cells, matcher, fallback) {
  const index = cells.findIndex(matcher);
  return index >= 0 ? index : fallback;
}

function isCodeHeader(value) {
  return ["学号", "编号", "序号"].includes(cleanImportCell(value));
}

function isStudentNameHeader(value) {
  return ["学生", "姓名", "学生姓名", "名字"].includes(cleanImportCell(value));
}

function isGenderHeader(value) {
  return ["性别", "男/女"].includes(cleanImportCell(value));
}

function isGroupHeader(value) {
  return ["分组", "小组", "组别", "组名"].includes(cleanImportCell(value));
}

function normalizeImportGender(value) {
  const text = cleanImportCell(value);
  if (text === "男" || text === "女") return text;
  return "";
}

function studentImportKey(code, name, gender) {
  const normalizedCode = cleanImportCell(code);
  if (normalizedCode) return `code:${normalizedCode}`;
  return `name:${cleanImportCell(name)}:${normalizeImportGender(gender)}`;
}

function safeArray(value) {
  return Array.isArray(value) ? value : [];
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
  <main v-if="activePage === 'login'" class="login-page">
    <section class="login-panel">
      <div class="mark">
        <Award />
      </div>
      <p class="eyebrow">一起学习，一起闪闪发光</p>
      <h1>班级积分系统</h1>
      <form @submit.prevent="login">
        <label>
          老师名称
          <input v-model="loginForm.name" autocomplete="off" placeholder="老师" />
        </label>
        <button class="primary wide" type="submit">
          <LogIn />
          进入班级乐园
        </button>
      </form>
    </section>
  </main>

  <div v-else class="app-shell">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand-logo"><Award /></div>
        <div>
          <p>快乐成长积分站</p>
          <strong>{{ selectedClass?.name || "我的班级" }}</strong>
        </div>
      </div>

      <button class="class-switch" type="button" @click="activePage = 'classes'">
        <ChevronLeft />
        我的班级
      </button>

      <nav v-if="selectedClassId" class="nav">
        <button v-for="page in pages" :key="page.key" :class="{ active: activePage === page.key }" type="button" @click="goPage(page.key)">
          <component :is="page.icon" />
          {{ page.label }}
        </button>
      </nav>

      <div class="sidebar-footer">
        <span>{{ user }}</span>
        <button class="ghost small" type="button" @click="logout">退出</button>
      </div>
    </aside>

    <section class="content" :class="{ 'classroom-content': activePage === 'classroom' }">
      <div v-if="loading" class="loading">加载中...</div>

      <section v-if="activePage === 'classes'" class="page">
        <div class="classes-page-actions">
          <button class="primary" type="button" @click="classModal = true">
            <Plus />
            创建班级
          </button>
        </div>
        <div v-if="!classes.length" class="empty">
          <Users />
          <strong>还没有班级</strong>
          <span>创建班级后，就可以添加学生、设置规则并开始课堂加减分。</span>
        </div>
        <div class="class-grid">
          <article v-for="item in classes" :key="item.id" class="class-card" @click="selectClass(item.id)">
            <div>
              <h2>{{ item.name }}</h2>
              <p>{{ item.teacher || "未填写老师" }}</p>
            </div>
            <div class="class-card-actions">
              <button class="ghost danger" type="button" @click.stop="deleteClass(item)"><Trash2 />删除</button>
              <button class="ghost" type="button">进入</button>
            </div>
          </article>
        </div>
      </section>

      <section v-if="activePage === 'home' && dashboard" class="page">
        <div class="stats-grid">
          <article class="stat"><span>学生人数</span><strong>{{ dashboard.studentCount }}</strong></article>
          <article class="stat"><span>班级总分</span><strong>{{ dashboard.totalScore }}</strong></article>
          <article class="stat"><span>今日记录</span><strong>{{ dashboard.todayRecords }}</strong></article>
          <article class="stat"><span>当前第一</span><strong>{{ dashboard.leader ? dashboard.leader.name : "暂无" }}</strong></article>
        </div>
        <div class="two-column">
          <section class="panel">
            <div class="panel-title">
              <h2>近期记录</h2>
              <button class="ghost small" type="button" @click="goPage('records')">查看全部</button>
            </div>
            <RecordList :records="dashboard.recentRecords" @undo="undoRecord" />
          </section>
          <section class="panel">
            <div class="panel-title">
              <h2>前三名</h2>
              <button class="ghost small" type="button" @click="goPage('ranking')">排行榜</button>
            </div>
            <div class="podium">
              <article v-for="(student, index) in topThree" :key="student.id">
                <span>{{ index + 1 }}</span>
                <strong>{{ student.name }}</strong>
                <em>{{ formatScore(student.score) }}</em>
              </article>
              <div v-if="!topThree.length" class="empty compact">暂无排行数据</div>
            </div>
          </section>
        </div>
      </section>

      <section v-if="activePage === 'classroom'" class="page classroom-page">
        <section class="classroom-toolbar">
          <div>
            <p class="eyebrow">先选择学生，再选择加分操作并确认提交</p>
            <h2>课堂加分</h2>
          </div>
          <div class="classroom-actions">
            <div class="search-box compact-search">
              <Search />
              <input v-model="studentKeyword" placeholder="搜索姓名或学号" />
            </div>
            <button class="primary" type="button" :disabled="!students.length" @click="openBatchScore(true)">
              <Users />
              全班加分
            </button>
          </div>
        </section>

        <section class="classroom-selection-bar" :class="{ active: batchSelectedCount }">
          <div>
            <strong v-if="batchSelectedCount">已选择 {{ batchSelectedCount }} 名学生</strong>
            <strong v-else>请点击学生卡片进行选择</strong>
            <span>{{ batchSelectedCount ? selectedClassroomStudents.map((student) => student.name).join("、") : "可连续选择多名学生" }}</span>
          </div>
          <div class="selection-actions">
            <button v-if="batchSelectedCount" class="ghost small" type="button" @click="clearClassroomSelection">清空选择</button>
            <button class="primary" type="button" :disabled="!batchSelectedCount" @click="openBatchScore(false)">
              <Sparkles />选择加分操作
            </button>
          </div>
        </section>

        <section class="student-groups" aria-label="分组学生积分卡片">
          <p class="drag-group-tip"><UsersRound />按住学生卡片，拖到其他小组即可调整分组</p>
          <section
            v-for="group in classroomGroups"
            :key="group.name"
            class="student-group-section"
            :class="{ 'drop-target': draggingStudentId && dragTargetGroup === group.name }"
            @dragover.prevent="dragTargetGroup = group.name"
            @drop.prevent="dropStudentOnGroup($event, group.name)"
          >
            <header class="student-group-title">
              <h3>{{ group.name }}</h3>
              <span>{{ group.items.length }} 人</span>
            </header>
            <div class="student-card-grid">
              <div v-if="!group.items.length" class="empty-group-drop">将学生拖到这里</div>
              <button
                v-for="item in group.items"
                :key="item.student.id"
                class="student-score-card"
                :class="{
                  selected: batchForm.studentIds.includes(String(item.student.id)),
                  dragging: draggingStudentId === item.student.id,
                }"
                draggable="true"
                :aria-pressed="batchForm.studentIds.includes(String(item.student.id))"
                :title="`点击选择 ${item.student.name}，拖动可调整分组`"
                type="button"
                @click="handleStudentCardClick(item.student)"
                @dragstart="startStudentDrag($event, item.student)"
                @dragend="finishStudentDrag"
              >
                <span class="student-no">{{ studentNumber(item.student, item.index) }}号</span>
                <span class="cartoon-avatar" :class="avatarClass(item.student, item.index)" aria-hidden="true">
                  <span class="avatar-ear left"></span>
                  <span class="avatar-ear right"></span>
                  <span class="avatar-hair-back"></span>
                  <span class="avatar-hair"></span>
                  <span class="avatar-accessory left"></span>
                  <span class="avatar-accessory right"></span>
                  <span class="avatar-face">
                    <span class="avatar-eye left"></span>
                    <span class="avatar-eye right"></span>
                    <span class="avatar-blush left"></span>
                    <span class="avatar-blush right"></span>
                    <span class="avatar-smile"></span>
                  </span>
                  <span class="avatar-body"><span class="avatar-collar"></span></span>
                </span>
                <strong>{{ item.student.name }}</strong>
                <em :class="{ negative: item.student.score < 0 }">{{ formatTotalScore(item.student.score) }}</em>
                <span v-if="batchForm.studentIds.includes(String(item.student.id))" class="student-selected-mark"><Check /></span>
              </button>
            </div>
          </section>
          <div v-if="!classroomStudents.length" class="empty compact">暂无匹配学生</div>
        </section>
      </section>

      <section v-if="activePage === 'students'" class="page">
        <section class="panel">
          <div class="panel-title">
            <h2>学生管理</h2>
            <div class="student-actions">
              <button class="secondary" type="button" @click="openImportStudents">
                <Upload />
                导入学生
              </button>
              <button class="primary" type="button" @click="openCreateStudent">
                <CirclePlus />
                新增学生
              </button>
            </div>
          </div>
          <div class="search-box">
            <Search />
            <input v-model="studentKeyword" placeholder="搜索姓名、分组、性别或学号" />
          </div>
          <div class="table-wrap">
            <table>
              <thead><tr><th>姓名</th><th>分组</th><th>性别</th><th>学号</th><th>积分</th><th>操作</th></tr></thead>
              <tbody>
                <tr v-for="student in filteredStudents" :key="student.id">
                  <td>{{ student.name }}</td>
                  <td>{{ student.groupName || "未分组" }}</td>
                  <td>{{ student.gender || "-" }}</td>
                  <td>{{ student.code || "-" }}</td>
                  <td><strong :class="{ negative: student.score < 0 }">{{ formatScore(student.score) }}</strong></td>
                  <td>
                    <div class="row-actions">
                      <button class="icon" type="button" title="编辑学生" @click="openEditStudent(student)"><Pencil /></button>
                      <button class="icon danger" type="button" title="删除学生" @click="deleteStudent(student)"><Trash2 /></button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </section>

      <section v-if="activePage === 'groups'" class="page">
        <section class="panel">
          <div class="panel-title">
            <div>
              <h2>小组管理</h2>
              <span>创建小组后，可在学生管理中为学生选择所属小组</span>
            </div>
            <button class="primary" type="button" @click="openCreateGroup">
              <CirclePlus />
              新增小组
            </button>
          </div>
          <div v-if="studentGroups.length" class="group-card-grid">
            <article v-for="(group, index) in studentGroups" :key="group.id" class="group-manage-card">
              <div class="group-card-icon" :class="`variant-${index % 4}`"><UsersRound /></div>
              <div class="group-card-content">
                <h3>{{ group.name }}</h3>
                <span>{{ group.studentCount }} 名学生</span>
              </div>
              <div class="group-card-actions">
                <button class="ghost small" type="button" @click="openEditGroup(group)"><Pencil />编辑</button>
                <button class="ghost small danger" type="button" @click="deleteStudentGroup(group)"><Trash2 />删除</button>
              </div>
            </article>
          </div>
          <div v-else class="empty compact">
            <UsersRound />
            <strong>还没有小组</strong>
            <span>可以创建星星组、月亮组等小组。</span>
          </div>
        </section>
      </section>

      <section v-if="activePage === 'rules'" class="page">
        <section class="panel">
          <div class="panel-title">
            <h2>积分规则</h2>
            <button class="primary" type="button" @click="ruleModal = true">
              <CirclePlus />
              新增规则
            </button>
          </div>
          <div class="rule-grid">
            <article v-for="rule in rules" :key="rule.id" class="rule-card" :class="{ disabled: !rule.enabled }">
              <div>
                <span>{{ rule.category || "未分类" }}</span>
                <h3>{{ rule.name }}</h3>
              </div>
              <strong :class="{ negative: rule.score < 0 }">{{ formatScore(rule.score) }}</strong>
              <button class="ghost small" type="button" @click="toggleRule(rule)">{{ rule.enabled ? "停用" : "启用" }}</button>
              <button class="icon danger" type="button" @click="deleteRule(rule)"><Trash2 /></button>
            </article>
          </div>
        </section>
      </section>

      <section v-if="activePage === 'records'" class="page">
        <section class="panel">
          <div class="panel-title">
            <h2>积分记录</h2>
            <div class="record-title-actions">
              <span>{{ records.length }} 条</span>
              <button class="secondary" type="button" :disabled="!records.length" @click="exportRecordsExcel">
                <Download />导出 Excel
              </button>
            </div>
          </div>
          <div class="record-filter-bar">
            <label>
              学生
              <select v-model="recordStudentId" @change="loadRecords">
                <option value="">全部学生</option>
                <option v-for="student in students" :key="student.id" :value="String(student.id)">
                  {{ student.name }}<template v-if="student.code">（{{ student.code }}）</template>
                </option>
              </select>
            </label>
            <label>
              开始日期
              <input v-model="recordStartDate" type="date" :max="recordEndDate || undefined" @change="loadRecords" />
            </label>
            <label>
              结束日期
              <input v-model="recordEndDate" type="date" :min="recordStartDate || undefined" @change="loadRecords" />
            </label>
            <label>
              记录类型
              <select v-model="recordFilter" @change="loadRecords">
                <option value="all">全部记录</option>
                <option value="today">今日记录</option>
                <option value="positive">只看加分</option>
                <option value="negative">只看扣分</option>
                <option value="undone">已撤销</option>
              </select>
            </label>
            <button class="ghost record-reset" type="button" @click="resetRecordFilters">重置筛选</button>
          </div>
          <RecordList :records="records" @undo="undoRecord" />
        </section>
      </section>

      <section v-if="activePage === 'ranking'" class="page">
        <section class="panel">
          <div class="panel-title">
            <h2>排行榜</h2>
          </div>
          <div class="ranking-filter-bar">
            <div class="ranking-quick-filters">
              <span>快捷筛选</span>
              <button
                v-for="scope in rankScopes"
                :key="scope.value"
                class="ghost small"
                :class="{ active: rankScope === scope.value }"
                type="button"
                @click="selectRankScope(scope.value)"
              >
                {{ scope.label }}
              </button>
            </div>
            <div class="ranking-date-filters">
              <label>
                开始日期
                <input v-model="rankStartDate" type="date" :max="rankEndDate || undefined" />
              </label>
              <label>
                结束日期
                <input v-model="rankEndDate" type="date" :min="rankStartDate || undefined" />
              </label>
              <button class="primary" :class="{ active: rankScope === 'custom' }" type="button" @click="applyCustomRankingDates">
                按日期筛选
              </button>
            </div>
          </div>
          <div class="table-wrap">
            <table>
              <thead><tr><th>排名</th><th>姓名</th><th>性别</th><th>学号</th><th>积分</th></tr></thead>
              <tbody>
                <tr v-for="(student, index) in ranking" :key="student.id">
                  <td><span class="rank">{{ index + 1 }}</span></td>
                  <td>{{ student.name }}</td>
                  <td>{{ student.gender || "-" }}</td>
                  <td>{{ student.code || "-" }}</td>
                  <td><strong :class="{ negative: student.score < 0 }">{{ formatScore(student.score) }}</strong></td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>
      </section>

      <section v-if="activePage === 'settings'" class="page">
        <section class="panel settings-panel">
          <div class="panel-title">
            <h2>系统设置</h2>
            <button class="secondary" type="button" @click="backupNow">
              <Download />
              立即备份
            </button>
          </div>
          <form class="form-grid" @submit.prevent="saveSettings">
            <label>
              学校名称
              <input v-model="settingForm.schoolName" />
            </label>
            <label>
              备份目录
              <input v-model="settingForm.backupDir" />
            </label>
            <label>
              自动备份间隔（分钟）
              <input v-model.number="settingForm.autoBackupMins" type="number" min="1" />
            </label>
            <label>
              保留备份数量
              <input v-model.number="settingForm.backupKeepCount" type="number" min="1" />
            </label>
            <button class="primary full" type="submit">
              <Save />
              保存设置
            </button>
          </form>
        </section>
      </section>
    </section>
  </div>

  <div v-if="classModal" class="modal-backdrop">
    <form class="modal" @submit.prevent="createClass">
      <header><h2>创建班级</h2><button class="icon" type="button" @click="classModal = false"><X /></button></header>
      <label>班级名称<input v-model="classForm.name" autocomplete="off" placeholder="如：三年级一班" /></label>
      <label>任课老师<input v-model="classForm.teacher" autocomplete="off" placeholder="可选" /></label>
      <button class="primary wide" type="submit"><Check />创建</button>
    </form>
  </div>

  <div v-if="studentModal" class="modal-backdrop">
    <form class="modal" @submit.prevent="saveStudent">
      <header><h2>{{ editingStudentId ? "编辑学生" : "新增学生" }}</h2><button class="icon" type="button" @click="closeStudentModal"><X /></button></header>
      <label>姓名<input v-model="studentForm.name" autocomplete="off" /></label>
      <label>
        分组
        <select v-model="studentForm.groupName">
          <option value="">未分组</option>
          <option v-for="group in studentGroups" :key="group.id" :value="group.name">{{ group.name }}</option>
        </select>
      </label>
      <label>
        性别
        <select v-model="studentForm.gender">
          <option value="">未填写</option>
          <option value="男">男</option>
          <option value="女">女</option>
        </select>
      </label>
      <label v-if="!editingStudentId">学号<input v-model="studentForm.code" autocomplete="off" /></label>
      <button class="primary wide" type="submit"><Check />保存</button>
    </form>
  </div>

  <div v-if="groupModal" class="modal-backdrop">
    <form class="modal" @submit.prevent="saveStudentGroup">
      <header>
        <h2>{{ editingGroupId ? "编辑小组" : "新增小组" }}</h2>
        <button class="icon" type="button" @click="closeGroupModal"><X /></button>
      </header>
      <label>小组名称<input v-model="groupForm.name" autocomplete="off" placeholder="如：星星组" /></label>
      <button class="primary wide" type="submit"><Check />保存</button>
    </form>
  </div>

  <div v-if="importModal" class="modal-backdrop">
    <form class="modal wide-modal" @submit.prevent="importStudents">
      <header><h2>导入学生</h2><button class="icon" type="button" @click="closeImportStudents"><X /></button></header>
      <div class="import-source">
        <label class="full">
          粘贴名单
          <textarea v-model="importForm.text" rows="8" placeholder="从 Excel 复制包含「学号、学生、性别、分组」的表格区域后粘贴到这里"></textarea>
        </label>
        <label class="file-picker">
          <Upload />
          选择 XLSX/CSV/TXT
          <input type="file" accept=".xlsx,.csv,.txt,.tsv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet,text/csv,text/plain" @change="handleImportFile" />
        </label>
      </div>
      <div class="import-summary">
        <span>解析 {{ importSummary.total }} 行</span>
        <span>可导入 {{ importSummary.ready }} 人</span>
        <span v-if="importSummary.duplicate">重复 {{ importSummary.duplicate }} 人</span>
        <span v-if="importSummary.invalid">无效 {{ importSummary.invalid }} 行</span>
      </div>
      <div class="table-wrap import-preview">
        <table>
          <thead><tr><th>行号</th><th>学号</th><th>姓名</th><th>性别</th><th>分组</th><th>状态</th></tr></thead>
          <tbody>
            <tr v-if="!importRows.length"><td colspan="6">暂无可预览内容</td></tr>
            <tr v-for="row in importRows" :key="row.line" :class="{ muted: row.duplicate || !row.valid }">
              <td>{{ row.line }}</td>
              <td>{{ row.code || "-" }}</td>
              <td>{{ row.name || "-" }}</td>
              <td>{{ row.gender || "-" }}</td>
              <td>{{ row.groupName || "未分组" }}</td>
              <td>{{ row.status }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <button class="primary wide" type="submit" :disabled="!importReadyRows.length"><Upload />确认导入</button>
    </form>
  </div>

  <div v-if="ruleModal" class="modal-backdrop">
    <form class="modal" @submit.prevent="createRule">
      <header><h2>新增规则</h2><button class="icon" type="button" @click="ruleModal = false"><X /></button></header>
      <label>规则名称<input v-model="ruleForm.name" autocomplete="off" placeholder="如：积极发言" /></label>
      <label>分值<input v-model.number="ruleForm.score" type="number" step="1" /></label>
      <label>分类<input v-model="ruleForm.category" autocomplete="off" /></label>
      <button class="primary wide" type="submit"><Check />保存</button>
    </form>
  </div>

  <div v-if="batchModal" class="modal-backdrop">
    <form class="modal wide-modal" @submit.prevent="createBatchRecords">
      <header><h2>选择加分操作</h2><button class="icon" type="button" @click="closeBatchScore"><X /></button></header>
      <div class="batch-selection-toolbar">
        <span>本次将为 {{ batchSelectedCount }} 名学生加分</span>
        <button class="ghost small" type="button" @click="closeBatchScore">返回修改选择</button>
      </div>
      <div class="selected-student-chips">
        <span v-for="student in selectedClassroomStudents" :key="student.id">{{ student.name }}</span>
      </div>
      <div class="score-operation-rules">
        <button
          v-for="rule in enabledRules"
          :key="rule.id"
          :class="{ active: batchForm.ruleId === String(rule.id) }"
          type="button"
          @click="applyRuleToForm(batchForm, rule.id)"
        >
          <span>{{ rule.name }}</span>
          <strong>{{ formatScore(rule.score) }}</strong>
        </button>
        <button :class="{ active: !batchForm.ruleId }" type="button" @click="batchForm.ruleId = ''">
          <span>自定义</span>
          <strong>{{ formatScore(batchForm.score) }}</strong>
        </button>
      </div>
      <div class="form-grid">
        <label>
          规则
          <select v-model="batchForm.ruleId" @change="applyRuleToForm(batchForm, batchForm.ruleId)">
            <option value="">自定义</option>
            <option v-for="rule in enabledRules" :key="rule.id" :value="String(rule.id)">
              {{ rule.name }}（{{ formatScore(rule.score) }}）
            </option>
          </select>
        </label>
        <label>分值<input v-model.number="batchForm.score" type="number" step="1" /></label>
        <label class="full">原因<input v-model="batchForm.reason" autocomplete="off" /></label>
        <label class="full">备注<textarea v-model="batchForm.note" rows="2"></textarea></label>
      </div>
      <button class="primary wide score-submit-button" type="submit" :disabled="!batchSelectedCount">
        <Check />确认提交，为 {{ batchSelectedCount }} 名学生加分
      </button>
    </form>
  </div>

  <div v-if="undoModal" class="modal-backdrop" @click.self="closeUndoModal">
    <form class="modal undo-modal" @submit.prevent="confirmUndoRecord" @keydown.esc="closeUndoModal">
      <header class="undo-modal-header">
        <div class="undo-modal-heading">
          <span class="undo-modal-icon"><RotateCcw /></span>
          <div>
            <h2>撤销积分记录</h2>
            <p>撤销后该分值将不再计入统计</p>
          </div>
        </div>
        <button class="icon" type="button" aria-label="关闭" :disabled="undoSubmitting" @click="closeUndoModal"><X /></button>
      </header>

      <div v-if="undoRecordTarget" class="undo-record-summary">
        <div>
          <strong>{{ undoRecordTarget.studentName }}</strong>
          <span>{{ undoRecordTarget.reason }}</span>
        </div>
        <em :class="{ negative: undoRecordTarget.score < 0 }">{{ formatScore(undoRecordTarget.score) }}</em>
        <p>{{ undoRecordTarget.ruleName }} · {{ formatDate(undoRecordTarget.createdAt) }}</p>
      </div>

      <label class="undo-reason-field">
        撤销原因
        <textarea
          v-model="undoReason"
          rows="3"
          maxlength="100"
          autofocus
          placeholder="请说明为什么撤销这条记录"
        ></textarea>
      </label>
      <div class="undo-reason-options" aria-label="常用撤销原因">
        <button
          v-for="reason in ['误操作', '重复记录', '分值有误']"
          :key="reason"
          :class="{ active: undoReason === reason }"
          type="button"
          @click="undoReason = reason"
        >
          {{ reason }}
        </button>
      </div>

      <div class="undo-modal-actions">
        <button class="ghost" type="button" :disabled="undoSubmitting" @click="closeUndoModal">取消</button>
        <button class="undo-confirm" type="submit" :disabled="undoSubmitting || !undoReason.trim()">
          <RotateCcw />{{ undoSubmitting ? "正在撤销..." : "确认撤销" }}
        </button>
      </div>
    </form>
  </div>

  <div v-if="message" class="toast">{{ message }}</div>
</template>
