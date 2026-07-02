<template>
  <BaseDialog :show="show" :title="t('admin.accounts.batchBedrock.title')" width="wide" @close="handleClose">
    <div class="space-y-5">
      <!-- API key -->
      <div>
        <label class="input-label">{{ t('admin.accounts.batchBedrock.apiKey') }}</label>
        <input v-model="apiKey" type="password" class="input" autocomplete="off" />
        <p class="input-hint">{{ t('admin.accounts.batchBedrock.apiKeyHint') }}</p>
      </div>

      <!-- Profiles -->
      <div>
        <label class="input-label">{{ t('admin.accounts.batchBedrock.profiles') }}</label>
        <div class="flex flex-col gap-2">
          <label class="flex items-center gap-2 cursor-pointer">
            <input v-model="profileGeo" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500" />
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('admin.accounts.batchBedrock.geoProfile') }}</span>
          </label>
          <label class="flex items-center gap-2 cursor-pointer">
            <input v-model="profileGlobal" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500" />
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('admin.accounts.batchBedrock.globalProfile') }}</span>
          </label>
          <label v-if="profileGeo && profileGlobal" class="ml-6 flex items-center gap-2 cursor-pointer">
            <input v-model="splitGroups" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500" />
            <span class="text-sm text-gray-600 dark:text-gray-400">{{ t('admin.accounts.batchBedrock.splitGroups') }}</span>
          </label>
        </div>
      </div>

      <!-- Regions -->
      <div>
        <div class="mb-2 flex items-center justify-between">
          <label class="input-label mb-0">{{ t('admin.accounts.batchBedrock.regions') }}</label>
          <div class="flex items-center gap-2 text-xs">
            <button type="button" class="text-primary-600 hover:underline" @click="selectAll">{{ t('admin.accounts.batchBedrock.selectAll') }}</button>
            <button type="button" class="text-primary-600 hover:underline" @click="selectNone">{{ t('admin.accounts.batchBedrock.selectNone') }}</button>
            <button type="button" class="text-primary-600 hover:underline" @click="invertSelection">{{ t('admin.accounts.batchBedrock.invert') }}</button>
            <span class="text-gray-500">{{ t('admin.accounts.batchBedrock.selectedCount', { count: selectedRegions.length }) }}</span>
          </div>
        </div>
        <div class="max-h-64 overflow-y-auto rounded-lg border border-gray-200 p-3 dark:border-dark-600">
          <div v-for="grp in AWS_BEDROCK_REGIONS" :key="grp.label" class="mb-3 last:mb-0">
            <div class="mb-1 text-xs font-semibold uppercase tracking-wide text-gray-400">{{ grp.label }}</div>
            <div class="grid grid-cols-1 gap-1 sm:grid-cols-2">
              <label v-for="r in grp.options" :key="r.code" class="flex items-center gap-2 cursor-pointer text-sm">
                <input
                  type="checkbox"
                  class="rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500"
                  :checked="isRegionSelected(r.code)"
                  @change="toggleRegion(r.code)"
                />
                <span class="font-mono text-gray-700 dark:text-gray-300">{{ r.code }}</span>
                <span class="text-gray-400">{{ r.city }} · {{ awsBedrockGeoFamily(r.code) }}.</span>
              </label>
            </div>
          </div>
        </div>
        <p class="input-hint">{{ t('admin.accounts.batchBedrock.nonBedrockRegionHint') }}</p>
      </div>

      <!-- Pool / load-balancing -->
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div v-if="!splitGroups">
          <label class="input-label">{{ t('admin.accounts.batchBedrock.group') }}</label>
          <select v-model="groupId" class="input">
            <option :value="null">{{ t('admin.accounts.batchBedrock.noGroup') }}</option>
            <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
          </select>
        </div>
        <template v-else>
          <div>
            <label class="input-label">{{ t('admin.accounts.batchBedrock.geoGroup') }}</label>
            <select v-model="geoGroupId" class="input">
              <option :value="null">{{ t('admin.accounts.batchBedrock.noGroup') }}</option>
              <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
            </select>
          </div>
          <div>
            <label class="input-label">{{ t('admin.accounts.batchBedrock.globalGroup') }}</label>
            <select v-model="globalGroupId" class="input">
              <option :value="null">{{ t('admin.accounts.batchBedrock.noGroup') }}</option>
              <option v-for="g in groups" :key="g.id" :value="g.id">{{ g.name }}</option>
            </select>
          </div>
        </template>
        <div>
          <label class="input-label">{{ t('admin.accounts.batchBedrock.namePrefix') }}</label>
          <input v-model="namePrefix" type="text" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.batchBedrock.priority') }}</label>
          <input v-model.number="priority" type="number" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.batchBedrock.concurrency') }}</label>
          <input v-model.number="concurrency" type="number" min="1" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.batchBedrock.loadFactor') }}</label>
          <input v-model.number="loadFactor" type="number" min="1" class="input" />
        </div>
      </div>
      <p class="input-hint">{{ t('admin.accounts.batchBedrock.groupHint') }}</p>
      <p v-if="noGroupSelected" class="text-xs text-amber-600 dark:text-amber-400">{{ t('admin.accounts.batchBedrock.noGroupWarning') }}</p>

      <!-- Advanced -->
      <label class="flex items-center gap-2 cursor-pointer">
        <input v-model="poolMode" type="checkbox" class="rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-500" />
        <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('admin.accounts.batchBedrock.poolMode') }}</span>
      </label>
      <p class="input-hint">{{ t('admin.accounts.batchBedrock.poolModeHint') }}</p>

      <!-- Result -->
      <div v-if="errorMessage" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-300">
        {{ errorMessage }}
      </div>
      <div v-if="result" class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-sm dark:border-dark-600 dark:bg-dark-700">
        <div class="font-medium">{{ t('admin.accounts.batchBedrock.resultSummary', { success: result.success, failed: result.failed }) }}</div>
        <div v-if="result.failures.length" class="mt-2">
          <div class="text-xs font-semibold text-gray-500">{{ t('admin.accounts.batchBedrock.resultFailures') }}</div>
          <ul class="mt-1 max-h-32 overflow-y-auto text-xs text-red-600 dark:text-red-400">
            <li v-for="f in result.failures" :key="f.name"><span class="font-mono">{{ f.name }}</span>: {{ f.error }}</li>
          </ul>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex items-center justify-between gap-3">
        <span class="text-sm text-gray-600 dark:text-gray-400">
          {{ selectedProfiles.length && selectedRegions.length
            ? t('admin.accounts.batchBedrock.willCreate', { count: willCreateCount, regions: selectedRegions.length, profiles: selectedProfiles.length })
            : t('admin.accounts.batchBedrock.selectRegionsProfiles') }}
        </span>
        <div class="flex gap-2">
          <button type="button" class="btn btn-secondary" @click="handleClose">{{ t('common.cancel') }}</button>
          <button type="button" class="btn btn-primary" :disabled="!canSubmit" @click="handleSubmit">
            {{ submitting ? t('admin.accounts.batchBedrock.importing') : t('admin.accounts.batchBedrock.import', { count: willCreateCount }) }}
          </button>
        </div>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { AWS_BEDROCK_REGIONS, awsBedrockGeoFamily, commercialBedrockRegionCodes } from '@/constants/account'
import { expandBedrockBatch, type BedrockProfile } from '@/utils/bedrockBatchExpand'

const props = defineProps<{ show: boolean; groups: Array<{ id: number; name: string }> }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'created'): void }>()
const { t } = useI18n()

const LARGE_BATCH_THRESHOLD = 50
const allCommercialCodes = commercialBedrockRegionCodes()

const apiKey = ref('')
const selectedRegions = ref<string[]>([...allCommercialCodes])
const profileGeo = ref(true)
const profileGlobal = ref(true)
const splitGroups = ref(false)
const groupId = ref<number | null>(null)
const geoGroupId = ref<number | null>(null)
const globalGroupId = ref<number | null>(null)
const priority = ref(50)
const concurrency = ref(5)
const loadFactor = ref(1)
const namePrefix = ref('bedrock')
const poolMode = ref(false)
const submitting = ref(false)
const result = ref<{ success: number; failed: number; failures: Array<{ name: string; error: string }> } | null>(null)
const errorMessage = ref('')

const selectedProfiles = computed<BedrockProfile[]>(() => {
  const p: BedrockProfile[] = []
  if (profileGeo.value) p.push('geo')
  if (profileGlobal.value) p.push('global')
  return p
})
const willCreateCount = computed(() => expandBedrockBatch(buildConfig()).length)
const geoGroupIds = computed<number[]>(() => {
  const id = splitGroups.value ? geoGroupId.value : groupId.value
  return id ? [id] : []
})
const globalGroupIds = computed<number[]>(() => {
  const id = splitGroups.value ? globalGroupId.value : groupId.value
  return id ? [id] : []
})
const noGroupSelected = computed(() =>
  splitGroups.value ? !geoGroupId.value || !globalGroupId.value : !groupId.value
)
const canSubmit = computed(
  () =>
    apiKey.value.trim().length > 0 &&
    selectedRegions.value.length > 0 &&
    selectedProfiles.value.length > 0 &&
    !submitting.value
)

const isRegionSelected = (code: string) => selectedRegions.value.includes(code)
const toggleRegion = (code: string) => {
  selectedRegions.value = isRegionSelected(code)
    ? selectedRegions.value.filter((c) => c !== code)
    : [...selectedRegions.value, code]
}
const selectAll = () => {
  selectedRegions.value = [...allCommercialCodes]
}
const selectNone = () => {
  selectedRegions.value = []
}
const invertSelection = () => {
  selectedRegions.value = allCommercialCodes.filter((c) => !selectedRegions.value.includes(c))
}

const buildConfig = () => ({
  apiKey: apiKey.value,
  regions: selectedRegions.value,
  profiles: selectedProfiles.value,
  namePrefix: namePrefix.value,
  priority: priority.value,
  concurrency: concurrency.value,
  loadFactor: loadFactor.value,
  poolMode: poolMode.value,
  geoGroupIds: geoGroupIds.value,
  globalGroupIds: globalGroupIds.value,
})

const handleClose = () => emit('close')

watch(
  () => props.show,
  (v) => {
    if (v) {
      result.value = null
      errorMessage.value = ''
      submitting.value = false
      apiKey.value = ''
      selectedRegions.value = [...allCommercialCodes]
      profileGeo.value = true
      profileGlobal.value = true
      splitGroups.value = false
      groupId.value = null
      geoGroupId.value = null
      globalGroupId.value = null
      priority.value = 50
      concurrency.value = 5
      loadFactor.value = 1
      namePrefix.value = 'bedrock'
      poolMode.value = false
    }
  }
)

watch([profileGeo, profileGlobal], ([geo, global]) => {
  if (!(geo && global)) splitGroups.value = false
})

const handleSubmit = async () => {
  if (!canSubmit.value) return
  errorMessage.value = ''
  result.value = null
  const accounts = expandBedrockBatch(buildConfig())
  if (accounts.length > LARGE_BATCH_THRESHOLD) {
    if (!window.confirm(t('admin.accounts.batchBedrock.confirmLarge', { count: accounts.length }))) return
  }
  submitting.value = true
  try {
    const res = await adminAPI.accounts.batchCreate(accounts)
    const failures = res.results
      .map((r, i) => ({ r, name: r.name ?? accounts[i]?.name ?? '' }))
      .filter((x) => !x.r.success)
      .map((x) => ({ name: x.name, error: x.r.error ?? 'unknown' }))
    result.value = { success: res.success, failed: res.failed, failures }
    emit('created')
    if (res.failed === 0) emit('close')
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } }; message?: string }
    errorMessage.value = err?.response?.data?.message ?? err?.message ?? String(e)
  } finally {
    submitting.value = false
  }
}
</script>
