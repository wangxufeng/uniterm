import { reactive, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { msg } from '../services/message'
import { CheckForUpdate, GetAppInfo } from '../../wailsjs/go/main/App'
import { useI18n } from '../i18n'
import { useSettingsStore } from '../stores/settingsStore'
import type { UpdateInfo } from '../types/settings'

function showUpdateNotification(info: UpdateInfo) {
  const { t } = useI18n()
  ElMessage({
    message: t('settings.foundNewVersion') + ': ' + info.latest + ' <a href="#" onclick="event.preventDefault();window.runtime.BrowserOpenURL(\'' + info.releaseUrl + '\')" style="color:inherit;text-decoration:underline;">' + t('settings.openRelease') + '</a>',
    dangerouslyUseHTMLString: true,
    type: 'success',
    duration: 0,
    showClose: true,
  })
}

const CHECK_TIMEOUT = 15000

const updateInfo = ref<UpdateInfo | null>(null)
const checking = ref(false)
const autoCheck = ref(true)

let timer: ReturnType<typeof setInterval> | null = null

function startTimer() {
  stopTimer()
  timer = setInterval(() => {
    checkForUpdate()
  }, 24 * 60 * 60 * 1000)
}

function stopTimer() {
  if (timer !== null) {
    clearInterval(timer)
    timer = null
  }
}

async function checkForUpdate(showStatus = false): Promise<UpdateInfo | null> {
  checking.value = true
  try {
    const info = await Promise.race([
      CheckForUpdate(),
      new Promise<never>((_, reject) =>
        setTimeout(() => reject(new Error('timeout')), CHECK_TIMEOUT)
      ),
    ])
    updateInfo.value = info
    if (info.hasUpdate) {
      showUpdateNotification(info)
    } else if (showStatus) {
      const { t } = useI18n()
      msg.success(t('settings.upToDate'))
    }
    return info
  } catch {
    if (showStatus) {
      const { t } = useI18n()
      msg.error(t('settings.checkUpdateFailed'))
    }
    return null
  } finally {
    checking.value = false
  }
}

// Sync autoCheck with settings store and manage timer
watch(autoCheck, (enabled) => {
  try {
    const settings = useSettingsStore()
    settings.settings.autoCheckUpdate = enabled
    settings.save()
  } catch { /* store may not be ready yet */ }
  if (enabled) {
    startTimer()
  } else {
    stopTimer()
  }
})

function initAutoCheck() {
  checking.value = false
  // Fetch current version immediately so About page shows it
  GetAppInfo().then(info => {
    if (!updateInfo.value) {
      updateInfo.value = { hasUpdate: false, current: info.version, latest: '', releaseUrl: '' }
    }
  }).catch(() => {})
  try {
    const settings = useSettingsStore()
    const v = settings.settings.autoCheckUpdate
    autoCheck.value = (v == null) ? true : v
  } catch { /* use default */ }
  if (autoCheck.value) {
    setTimeout(() => checkForUpdate(), 5000)
    startTimer()
  }
}

const state = reactive({
  updateInfo,
  checking,
  autoCheck,
  checkForUpdate,
  initAutoCheck,
})

export function useUpdateCheck() {
  return state
}
