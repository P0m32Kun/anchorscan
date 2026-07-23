<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue';

type Zone = { id: string; name: string };
type FieldMessage = { field: string; message: string };

type FormValues = {
  zone_id: string;
  target: string;
  exclude_targets: string;
  ports: string;
  exclude_ports: string;
  profile: string;
  label: string;
  access_point: string;
  tester_ip: string;
  notes: string;
  rustscan_args: string;
  nmap_args: string;
  httpx_args: string;
  nuclei_args: string;
};

const props = defineProps<{
  projectId: string;
  zones: Zone[];
  form: FormValues;
  artifactRoot: string;
  highriskPorts: string;
  isRerun: boolean;
  errors: FieldMessage[];
  warnings: FieldMessage[];
  defaultZoneId: string;
}>();

const form = ref({ ...props.form });
const artifactRoot = ref(props.artifactRoot);
const submitting = ref(false);
const optionalOpen = ref(false);
const errorSummary = ref<HTMLElement>();
const optionalFields = ['label', 'exclude_targets', 'exclude_ports', 'notes', 'rustscan_args', 'nmap_args', 'httpx_args', 'nuclei_args', 'artifact_root'];
const formFields = ['zone_id', 'target', 'exclude_targets', 'ports', 'exclude_ports', 'profile', 'label', 'access_point', 'tester_ip', 'notes', 'rustscan_args', 'nmap_args', 'httpx_args', 'nuclei_args', 'artifact_root'];

const optionalChangedCount = computed(() => {
  const values = [...optionalFields.slice(0, -1).map((field) => form.value[field as keyof FormValues]), artifactRoot.value];
  return values.filter((value) => value.trim() !== '').length;
});

const firstErrorField = computed(() => props.errors[0]?.field);

function fieldError(field: string) {
  return props.errors.find((error) => error.field === field)?.message;
}

function insertHighriskPorts() {
  form.value.ports = props.highriskPorts;
}

function handleSubmit(event: Event) {
  const element = event.currentTarget as HTMLFormElement;
  if (submitting.value) {
    event.preventDefault();
    return;
  }
  if (!element.checkValidity()) {
    event.preventDefault();
    element.reportValidity();
    return;
  }
  submitting.value = true;
}

onMounted(() => {
  if (props.zones.length === 1 && !form.value.zone_id) form.value.zone_id = props.defaultZoneId;
  if (optionalChangedCount.value > 0 || props.errors.some(({ field }) => optionalFields.includes(field))) optionalOpen.value = true;
  if (firstErrorField.value) {
    nextTick(() => {
      if (formFields.includes(firstErrorField.value!)) document.getElementsByName(firstErrorField.value!)[0]?.focus();
      else errorSummary.value?.focus();
    });
  }
});
</script>

<template>
  <div class="scan-create">
    <div v-if="errors.length" ref="errorSummary" class="alert alert-error" role="alert" tabindex="-1">
      <strong>预检失败</strong>
      <ul>
        <li v-for="error in errors" :key="`${error.field}:${error.message}`">{{ error.field }}: {{ error.message }}</li>
      </ul>
    </div>
    <div v-if="warnings.length" class="alert alert-warning" role="alert">
      <strong>预检提醒</strong>
      <ul>
        <li v-for="warning in warnings" :key="`${warning.field}:${warning.message}`">{{ warning.field }}: {{ warning.message }}</li>
      </ul>
    </div>
    <p v-if="isRerun" class="alert alert-warning">这是一次重新运行。请检查并提交下方参数后，系统才会创建一个新的扫描任务。</p>

    <form class="form-grid" method="post" action="/scan" @submit="handleSubmit">
      <input type="hidden" name="project_id" :value="projectId">

      <label>
        <span>网络分区 (Zone) <span class="required">*</span></span>
        <select v-model="form.zone_id" name="zone_id" required :aria-describedby="fieldError('zone_id') ? 'error-zone_id' : undefined">
          <option value="">请选择 Zone</option>
          <option v-for="zone in zones" :key="zone.id" :value="zone.id">{{ zone.name }} ({{ zone.id }})</option>
        </select>
        <p v-if="fieldError('zone_id')" id="error-zone_id" class="field-error">{{ fieldError('zone_id') }}</p>
      </label>

      <label>
        <span>扫描档位 <span class="required">*</span></span>
        <select v-model="form.profile" name="profile" required :aria-describedby="fieldError('profile') ? 'error-profile' : undefined">
          <option value="slow">slow (轻载/低速率)</option>
          <option value="normal">normal (均衡/默认值)</option>
          <option value="fast">fast (极速/多路并发)</option>
        </select>
        <p v-if="fieldError('profile')" id="error-profile" class="field-error">{{ fieldError('profile') }}</p>
      </label>

      <label class="full-width">
        <span>目标资产 <span class="required">*</span></span>
        <textarea v-model="form.target" name="target" rows="4" required placeholder="支持 IP、CIDR(网段)或自定义范围，多目标用英文逗号或换行分隔" :aria-describedby="fieldError('target') ? 'error-target' : undefined" />
        <p v-if="fieldError('target')" id="error-target" class="field-error">{{ fieldError('target') }}</p>
      </label>

      <label class="full-width">
        <span>端口范围 <span class="required">*</span></span>
        <textarea v-model="form.ports" name="ports" rows="3" required placeholder="支持: top1000、100-1000 或 80,443,8080" :aria-describedby="fieldError('ports') ? 'error-ports' : undefined" />
        <p class="meta-line">端口格式保持 rustscan 习惯：top1000 = --top；100-1000 = --range；80,443,8080 = --ports。不支持 full/highrisk，需全端口请填 1-65535。</p>
        <button class="link-button" type="button" @click="insertHighriskPorts">＋ 插入高危端口列表</button>
        <p v-if="fieldError('ports')" id="error-ports" class="field-error">{{ fieldError('ports') }}</p>
      </label>

      <label>
        <span>测试设备接入点 <span class="required">*</span></span>
        <input v-model="form.access_point" name="access_point" required placeholder="XX 屏柜/xxx 交换机" :aria-describedby="fieldError('access_point') ? 'error-access_point' : undefined">
        <p v-if="fieldError('access_point')" id="error-access_point" class="field-error">{{ fieldError('access_point') }}</p>
      </label>

      <label>
        <span>测试设备 IP <span class="required">*</span></span>
        <input v-model="form.tester_ip" name="tester_ip" required placeholder="例如：10.0.0.5" :aria-describedby="fieldError('tester_ip') ? 'error-tester_ip' : undefined">
        <p v-if="fieldError('tester_ip')" id="error-tester_ip" class="field-error">{{ fieldError('tester_ip') }}</p>
      </label>

      <details :open="optionalOpen" class="details-block full-width" data-scan-create-options @toggle="optionalOpen = ($event.currentTarget as HTMLDetailsElement).open">
        <summary>可选信息与高级配置<span v-if="optionalChangedCount"> · 已修改 {{ optionalChangedCount }} 项</span></summary>
        <div class="form-grid project-scan-detail-body">
          <label>
            <span>扫描标签</span>
            <input v-model="form.label" name="label" placeholder="例如：核心交换机接入段">
          </label>
          <label>
            <span>备注</span>
            <textarea v-model="form.notes" name="notes" rows="2" placeholder="本次扫描的特殊说明" />
          </label>
          <label class="full-width">
            <span>排除目标</span>
            <textarea v-model="form.exclude_targets" name="exclude_targets" rows="2" placeholder="不在本次扫描范围内的 IP、CIDR 或范围" />
          </label>
          <label class="full-width">
            <span>排除端口</span>
            <textarea v-model="form.exclude_ports" name="exclude_ports" rows="2" placeholder="例如：22,3389 或 8000-8100" />
          </label>
          <label>
            <span>Rustscan 参数</span>
            <input v-model="form.rustscan_args" name="rustscan_args" placeholder="例如: --ulimit 5000">
          </label>
          <label>
            <span>Nmap 参数</span>
            <input v-model="form.nmap_args" name="nmap_args" placeholder="例如: --min-rate 300 -sV">
          </label>
          <label>
            <span>Httpx 参数</span>
            <input v-model="form.httpx_args" name="httpx_args" placeholder="例如: -title -content-length">
          </label>
          <label>
            <span>Nuclei 参数</span>
            <input v-model="form.nuclei_args" name="nuclei_args" placeholder="例如: -tags cve,vuln">
          </label>
          <label class="full-width">
            <span>Artifact 根目录</span>
            <input v-model="artifactRoot" name="artifact_root" placeholder="留空则与 report.json 放在同一个 run 目录">
          </label>
        </div>
      </details>

      <div class="form-actions form-submit-row full-width">
        <button class="button button-primary primary-submit" type="submit" :disabled="submitting">
          {{ submitting ? '正在创建扫描…' : '立即启动引擎扫描' }}
        </button>
      </div>
    </form>
  </div>
</template>

<style scoped>
.alert ul { margin: 0.45rem 0 0; padding-left: 1.25rem; }
.details-block summary span { color: var(--muted); font-size: 0.8rem; font-weight: 500; }
.field-error { color: var(--danger); font-size: 0.8rem; margin: 0; }
</style>
