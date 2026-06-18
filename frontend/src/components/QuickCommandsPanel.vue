<template>
  <div class="quick-commands-panel">
    <!-- Top toolbar -->
    <div class="qc-toolbar">
      <span class="qc-title">{{ t('quickCommands.title') }}</span>
      <div class="qc-toolbar-actions">
        <button class="qc-toolbar-btn" @click="addGroup" :title="t('quickCommands.addGroup')">
          <FolderPlus :size="15" />
        </button>
        <button class="qc-toolbar-btn" @click="addCommand()" :title="t('quickCommands.addCommand')">
          <Plus :size="15" />
        </button>
      </div>
    </div>

    <!-- Command list -->
    <div class="qc-list">
      <template v-for="group in store.groups" :key="group.id">
        <div
          class="qc-group-header"
          @click="toggleGroup(group.id)"
          @contextmenu.prevent="onGroupContextMenu($event, group)"
        >
          <component :is="expandedGroups.has(group.id) ? ChevronDown : ChevronRight" :size="14" class="qc-chevron" />
          <span class="qc-group-name">{{ group.name }}</span>
          <span class="qc-group-count">({{ getGroupCommandCount(group.id) }})</span>
        </div>

        <template v-if="expandedGroups.has(group.id)">
          <div
            v-for="cmd in store.getCommandsByGroup(group.id)"
            :key="cmd.id"
            class="qc-item"
            :class="{ selected: selectedId === cmd.id }"
            @click="selectCommand(cmd.id)"
            @dblclick="runCommand(cmd)"
            @contextmenu.prevent="onCommandContextMenu($event, cmd)"
            @mouseenter="hoveredId = cmd.id"
            @mouseleave="hoveredId = null"
          >
            <div class="qc-item-content">
              <div v-if="cmd.name" class="qc-item-name">{{ cmd.name }}</div>
              <div class="qc-item-cmd" :class="{ 'qc-item-cmd-only': !cmd.name }">{{ cmd.command }}</div>
            </div>
            <div v-if="selectedId === cmd.id || hoveredId === cmd.id" class="qc-item-actions">
              <button class="qc-action-btn run" @click.stop="runCommand(cmd)" :title="t('quickCommands.run')">
                <Play :size="14" />
              </button>
              <button class="qc-action-btn paste" @click.stop="pasteCommand(cmd)" :title="t('quickCommands.paste')">
                <Clipboard :size="14" />
              </button>
            </div>
          </div>
        </template>
      </template>

      <!-- Ungrouped commands -->
      <template v-if="store.getCommandsByGroup(undefined).length > 0">
        <div class="qc-group-header ungrouped">
          <span class="qc-group-name">{{ t('quickCommands.noGroup') }}</span>
        </div>
        <div
          v-for="cmd in store.getCommandsByGroup(undefined)"
          :key="cmd.id"
          class="qc-item"
          :class="{ selected: selectedId === cmd.id }"
          @click="selectCommand(cmd.id)"
          @dblclick="runCommand(cmd)"
          @contextmenu.prevent="onCommandContextMenu($event, cmd)"
          @mouseenter="hoveredId = cmd.id"
          @mouseleave="hoveredId = null"
        >
          <div class="qc-item-content">
            <div v-if="cmd.name" class="qc-item-name">{{ cmd.name }}</div>
            <div class="qc-item-cmd" :class="{ 'qc-item-cmd-only': !cmd.name }">{{ cmd.command }}</div>
          </div>
          <div v-if="selectedId === cmd.id || hoveredId === cmd.id" class="qc-item-actions">
            <button class="qc-action-btn run" @click.stop="runCommand(cmd)" :title="t('quickCommands.run')">
              <Play :size="14" />
            </button>
            <button class="qc-action-btn paste" @click.stop="pasteCommand(cmd)" :title="t('quickCommands.paste')">
              <Clipboard :size="14" />
            </button>
          </div>
        </div>
      </template>

      <!-- Empty state -->
      <div v-if="store.commands.length === 0" class="qc-empty">
        {{ t('quickCommands.empty') }}
      </div>
    </div>

    <!-- Right-click menu: Command -->
    <div
      v-show="cmdContextMenu.visible"
      class="qc-context-menu"
      :style="{ left: cmdContextMenu.x + 'px', top: cmdContextMenu.y + 'px' }"
      @click.stop
    >
      <div class="menu-item" @click="editCommand(cmdContextMenu.cmd!)">{{ t('quickCommands.editCommand') }}</div>
      <div class="menu-item danger" @click="deleteCommand(cmdContextMenu.cmd!)">{{ t('quickCommands.deleteCommand') }}</div>
    </div>

    <!-- Right-click menu: Group -->
    <div
      v-show="groupContextMenu.visible"
      class="qc-context-menu"
      :style="{ left: groupContextMenu.x + 'px', top: groupContextMenu.y + 'px' }"
      @click.stop
    >
      <div class="menu-item" @click="addCommand(groupContextMenu.group?.id)">{{ t('quickCommands.addCommand') }}</div>
      <div class="menu-item" @click="renameGroup(groupContextMenu.group!)">{{ t('quickCommands.renameGroup') }}</div>
      <div class="menu-item danger" @click="deleteGroupDialog(groupContextMenu.group!)">{{ t('quickCommands.deleteGroup') }}</div>
    </div>

    <!-- Delete group dialog -->
    <el-dialog
      v-model="deleteGroupDialogVisible"
      :title="t('quickCommands.deleteGroupTitle')"
      width="400px"
      :close-on-click-modal="false"
    >
      <p>{{ t('quickCommands.deleteGroupDesc') }}</p>
      <div class="delete-group-actions">
        <el-button @click="doDeleteGroup(false)">{{ t('quickCommands.moveToUngrouped') }}</el-button>
        <el-button type="danger" @click="doDeleteGroup(true)">{{ t('quickCommands.deleteCommands') }}</el-button>
      </div>
    </el-dialog>

    <!-- Group name dialog (add + rename) -->
    <el-dialog
      v-model="groupNameDialogVisible"
      :title="renamingGroup ? t('quickCommands.renameGroup') : t('quickCommands.addGroup')"
      width="360px"
      :close-on-click-modal="false"
    >
      <el-input v-model="groupNameInput" :placeholder="t('quickCommands.groupName')" maxlength="30" @keyup.enter="doSaveGroupName" />
      <template #footer>
        <el-button @click="groupNameDialogVisible = false">{{ t('quickCommands.cancel') }}</el-button>
        <el-button type="primary" :disabled="!groupNameInput.trim()" @click="doSaveGroupName">
          {{ t('quickCommands.save') }}
        </el-button>
      </template>
    </el-dialog>

    <!-- Command edit dialog -->
    <QuickCommandEditDialog
      v-model="editDialogVisible"
      :editing-id="editingCmdId"
      :initial-name="editingCmdName"
      :initial-command="editingCmdCommand"
      :initial-group-id="editingCmdGroupId"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import {
  FolderPlus, Plus, Play, Clipboard,
  ChevronDown, ChevronRight
} from '@lucide/vue'
import { useQuickCommandStore, type QuickCommand, type QuickCommandGroup } from '../stores/quickCommandStore'
import { useTabStore } from '../stores/tabStore'
import { usePanelStore } from '../stores/panelStore'
import { SessionWrite } from '../../wailsjs/go/main/App'
import { useI18n } from '../i18n'
import QuickCommandEditDialog from './QuickCommandEditDialog.vue'

const { t } = useI18n()
const store = useQuickCommandStore()
const tabStore = useTabStore()
const panelStore = usePanelStore()

const selectedId = ref<string | null>(null)
const hoveredId = ref<string | null>(null)
const expandedGroups = ref<Set<string>>(new Set())

const cmdContextMenu = ref<{ visible: boolean; x: number; y: number; cmd: QuickCommand | null }>({ visible: false, x: 0, y: 0, cmd: null })
const groupContextMenu = ref<{ visible: boolean; x: number; y: number; group: QuickCommandGroup | null }>({ visible: false, x: 0, y: 0, group: null })

const deleteGroupDialogVisible = ref(false)
const deletingGroup = ref<QuickCommandGroup | null>(null)

const groupNameDialogVisible = ref(false)
const groupNameInput = ref('')
const renamingGroup = ref<QuickCommandGroup | null>(null)

const editDialogVisible = ref(false)
const editingCmdId = ref<string | undefined>(undefined)
const editingCmdName = ref<string | undefined>(undefined)
const editingCmdCommand = ref('')
const editingCmdGroupId = ref<string | undefined>(undefined)

onMounted(async () => {
  await store.load()
  store.groups.forEach(g => expandedGroups.value.add(g.id))
  document.addEventListener('click', closeContextMenus)
})

onUnmounted(() => {
  document.removeEventListener('click', closeContextMenus)
})

function closeContextMenus() {
  cmdContextMenu.value.visible = false
  groupContextMenu.value.visible = false
}

function toggleGroup(id: string) {
  if (expandedGroups.value.has(id)) expandedGroups.value.delete(id)
  else expandedGroups.value.add(id)
}

function getGroupCommandCount(groupId: string): number {
  return store.getCommandsByGroup(groupId).length
}

function selectCommand(id: string) {
  selectedId.value = id
}

function getActiveSessionId(): string | null {
  const activeTabId = tabStore.activeTabId
  if (!activeTabId) return null
  const tab = tabStore.tabs.find(t => t.id === activeTabId)
  if (!tab) return null
  const activePanelId = tab.type === 'workspace' ? tab.activePanelId : (tab.type === 'terminal' ? tab.panelId : null)
  if (!activePanelId) return null
  const panel = panelStore.getPanel(activePanelId)
  if (!panel?.sessionId) return null
  return panel.sessionId
}

async function sendCommand(cmd: QuickCommand, mode: 'run' | 'paste') {
  const sid = getActiveSessionId()
  if (!sid) return
  if (mode === 'paste') {
    SessionWrite(sid, cmd.command)
    return
  }
  let text = cmd.command
  if (!text.endsWith('\n')) text += '\n'
  const lines = text.split('\n').filter(l => l.length > 0)
  for (let i = 0; i < lines.length; i++) {
    SessionWrite(sid, lines[i] + '\n')
    if (i < lines.length - 1) await new Promise(r => setTimeout(r, 100))
  }
}

function runCommand(cmd: QuickCommand) { sendCommand(cmd, 'run') }
function pasteCommand(cmd: QuickCommand) { sendCommand(cmd, 'paste') }

function onCommandContextMenu(e: MouseEvent, cmd: QuickCommand) {
  cmdContextMenu.value = { visible: true, x: e.clientX, y: e.clientY, cmd }
}
function onGroupContextMenu(e: MouseEvent, group: QuickCommandGroup) {
  groupContextMenu.value = { visible: true, x: e.clientX, y: e.clientY, group }
}

function editCommand(cmd: QuickCommand) {
  editingCmdId.value = cmd.id
  editingCmdName.value = cmd.name
  editingCmdCommand.value = cmd.command
  editingCmdGroupId.value = cmd.groupId
  editDialogVisible.value = true
  cmdContextMenu.value.visible = false
}

function deleteCommand(cmd: QuickCommand) {
  store.deleteCommand(cmd.id)
  if (selectedId.value === cmd.id) selectedId.value = null
  cmdContextMenu.value.visible = false
}

function addCommand(groupId?: string) {
  editingCmdId.value = undefined
  editingCmdName.value = undefined
  editingCmdCommand.value = ''
  editingCmdGroupId.value = groupId
  editDialogVisible.value = true
  groupContextMenu.value.visible = false
}

function addGroup() {
  renamingGroup.value = null
  groupNameInput.value = ''
  groupNameDialogVisible.value = true
}

function renameGroup(group: QuickCommandGroup) {
  renamingGroup.value = group
  groupNameInput.value = group.name
  groupNameDialogVisible.value = true
  groupContextMenu.value.visible = false
}

function doSaveGroupName() {
  const name = groupNameInput.value.trim()
  if (!name) return
  if (renamingGroup.value) store.renameGroup(renamingGroup.value.id, name)
  else store.addGroup(name)
  groupNameDialogVisible.value = false
}

function deleteGroupDialog(group: QuickCommandGroup) {
  deletingGroup.value = group
  deleteGroupDialogVisible.value = true
  groupContextMenu.value.visible = false
}

function doDeleteGroup(deleteCommands: boolean) {
  if (deletingGroup.value) store.deleteGroup(deletingGroup.value.id, deleteCommands)
  deleteGroupDialogVisible.value = false
  deletingGroup.value = null
}
</script>

<style scoped>
.quick-commands-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.qc-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
}

.qc-title {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.qc-toolbar-actions {
  display: flex;
  gap: 2px;
}

.qc-toolbar-btn {
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
}

.qc-toolbar-btn:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.qc-list {
  flex: 1;
  overflow-y: auto;
  padding: 4px 0;
}

.qc-group-header {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 12px;
  cursor: pointer;
  user-select: none;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.qc-group-header:hover {
  background: var(--bg-hover);
}

.qc-group-header.ungrouped {
  color: var(--text-muted);
  font-weight: 500;
}

.qc-chevron {
  flex-shrink: 0;
  color: var(--text-muted);
}

.qc-group-name {
  flex: 1;
}

.qc-group-count {
  color: var(--text-muted);
  font-weight: 400;
}

.qc-item {
  display: flex;
  align-items: center;
  padding: 4px 12px 4px 28px;
  cursor: pointer;
  gap: 4px;
  min-height: 36px;
}

.qc-item:hover {
  background: var(--bg-hover);
}

.qc-item.selected {
  background: var(--bg-active, rgba(34, 211, 238, 0.08));
}

.qc-item-content {
  flex: 1;
  min-width: 0;
  line-height: 1.4;
}

.qc-item-name {
  font-size: 12px;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.qc-item-cmd {
  font-size: 12px;
  color: var(--text-muted);
  font-family: var(--font-mono, 'Consolas', 'Courier New', monospace);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.qc-item-cmd-only {
  font-size: 12px;
}

.qc-item-actions {
  display: flex;
  gap: 2px;
  flex-shrink: 0;
}

.qc-action-btn {
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  color: var(--text-muted);
  background: transparent;
}

.qc-action-btn:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.qc-action-btn.run:hover {
  color: var(--success-color, #22c55e);
}

.qc-action-btn.paste:hover {
  color: var(--accent-color, #22d3ee);
}

.qc-empty {
  padding: 24px 12px;
  text-align: center;
  color: var(--text-muted);
  font-size: 12px;
}

.qc-context-menu {
  position: fixed;
  z-index: 9999;
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  box-shadow: var(--shadow-lg);
  padding: 4px;
  min-width: 140px;
}

.qc-context-menu .menu-item {
  padding: 6px 10px;
  font-size: 12px;
  border-radius: 4px;
  cursor: pointer;
  color: var(--text-primary);
}

.qc-context-menu .menu-item:hover {
  background: var(--bg-hover);
}

.qc-context-menu .menu-item.danger {
  color: var(--danger-color, #f56c6c);
}

.delete-group-actions {
  display: flex;
  gap: 8px;
  margin-top: 12px;
}
</style>
