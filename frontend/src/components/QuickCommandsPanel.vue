<template>
  <div class="quick-commands-panel">
    <!-- Toolbar: search + actions -->
    <div class="qc-toolbar">
      <el-input
        v-model="searchQuery"
        :placeholder="t('quickCommands.searchPlaceholder')"
        clearable
        size="small"
        class="qc-search-input"
      />
      <button class="qc-icon-btn" @click="addGroup" :title="t('quickCommands.addGroup')">
        <FolderPlus :size="15" />
      </button>
      <button class="qc-icon-btn" @click="addCommand()" :title="t('quickCommands.addCommand')">
        <Plus :size="15" />
      </button>
    </div>

    <!-- Command list -->
    <div class="qc-list" ref="listRef" tabindex="0" @keydown="onListKeydown">
      <template v-for="group in store.groups" :key="group.id">
        <div
          class="qc-group-header"
          @click="toggleGroup(group.id)"
          @contextmenu.prevent="onGroupContextMenu($event, group)"
        >
          <span class="qc-group-arrow">
            <el-icon v-if="expandedGroups.has(group.id)"><ChevronDown :size="14" /></el-icon>
            <el-icon v-else><ChevronRight :size="14" /></el-icon>
          </span>
          <span class="qc-group-name">{{ group.name }}</span>
          <span class="qc-group-count">({{ getGroupCommandCount(group.id) }})</span>
        </div>

        <template v-if="expandedGroups.has(group.id)">
          <div
            v-for="cmd in store.getCommandsByGroup(group.id).filter(matchesSearch)"
            :key="cmd.id"
            class="qc-item indented"
            :class="{ active: selectedId === cmd.id }"
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

      <!-- Flat ungrouped commands (only when no real groups exist) -->
      <template v-if="store.groups.length === 0">
        <div
          v-for="cmd in store.getCommandsByGroup(undefined).filter(matchesSearch)"
          :key="cmd.id"
          class="qc-item"
          :class="{ active: selectedId === cmd.id }"
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

      <!-- Virtual (No Group) group - only when real groups exist -->
      <template v-if="store.groups.length > 0 && store.getCommandsByGroup(undefined).filter(matchesSearch).length > 0">
        <div
          class="qc-group-header"
          @click="toggleGroup('__ungrouped__')"
        >
          <span class="qc-group-arrow">
            <el-icon v-if="expandedGroups.has('__ungrouped__')"><ChevronDown :size="14" /></el-icon>
            <el-icon v-else><ChevronRight :size="14" /></el-icon>
          </span>
          <span class="qc-group-name">{{ t('quickCommands.noGroup') }}</span>
        </div>
        <template v-if="expandedGroups.has('__ungrouped__')">
          <div
            v-for="cmd in store.getCommandsByGroup(undefined).filter(matchesSearch)"
            :key="cmd.id"
            class="qc-item indented"
            :class="{ active: selectedId === cmd.id }"
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
const focusedId = ref<string | null>(null)
const listRef = ref<HTMLDivElement | null>(null)
const hoveredId = ref<string | null>(null)
const searchQuery = ref('')
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
  expandedGroups.value.add('__ungrouped__')
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

function matchesSearch(cmd: QuickCommand): boolean {
  if (!searchQuery.value.trim()) return true
  const q = searchQuery.value.toLowerCase()
  if (cmd.name && cmd.name.toLowerCase().includes(q)) return true
  if (cmd.command.toLowerCase().includes(q)) return true
  return false
}

function getAllVisibleIds(): string[] {
  const ids: string[] = []
  for (const g of store.groups) {
    if (expandedGroups.value.has(g.id)) {
      for (const c of store.getCommandsByGroup(g.id).filter(matchesSearch)) {
        ids.push(c.id)
      }
    }
  }
  // ungrouped
  if (store.groups.length > 0) {
    if (expandedGroups.value.has('__ungrouped__')) {
      for (const c of store.getCommandsByGroup(undefined).filter(matchesSearch)) {
        ids.push(c.id)
      }
    }
  } else {
    for (const c of store.getCommandsByGroup(undefined).filter(matchesSearch)) {
      ids.push(c.id)
    }
  }
  return ids
}

function onListKeydown(e: KeyboardEvent) {
  const ids = getAllVisibleIds()
  if (ids.length === 0) return
  const idx = ids.indexOf(focusedId.value || '')

  if (e.key === 'ArrowDown') {
    e.preventDefault()
    const nextIdx = idx >= 0 && idx < ids.length - 1 ? idx + 1 : 0
    focusedId.value = ids[nextIdx]
    selectedId.value = ids[nextIdx]
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    const prevIdx = idx > 0 ? idx - 1 : ids.length - 1
    focusedId.value = ids[prevIdx]
    selectedId.value = ids[prevIdx]
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (focusedId.value) {
      const cmd = store.commands.find(c => c.id === focusedId.value)
      if (cmd) runCommand(cmd)
    }
  } else if (e.key === 'Delete') {
    e.preventDefault()
    if (focusedId.value) {
      const cmd = store.commands.find(c => c.id === focusedId.value)
      if (cmd) deleteCommand(cmd)
    }
  }
}

function selectCommand(id: string) {
  selectedId.value = id
  focusedId.value = id
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
  if (focusedId.value === cmd.id) focusedId.value = null
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
  gap: 4px;
  padding: 0 10px 6px;
  flex-shrink: 0;
  border-bottom: 1px solid var(--border-color);
}

.qc-search-input {
  flex: 1;
  min-width: 0;
}

.qc-icon-btn {
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
  flex-shrink: 0;
}

.qc-icon-btn:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.qc-list {
  flex: 1;
  overflow-y: auto;
  padding: 0 8px 8px;
}

.qc-group-header {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 10px 6px 6px;
  cursor: pointer;
  user-select: none;
  border-radius: var(--radius-sm);
  transition: background 0.12s ease;
  font-family: var(--font-ui);
  font-size: 12px;
  color: var(--text-secondary);
}

.qc-group-header:hover {
  background: var(--bg-hover);
}

.qc-group-arrow {
  display: inline-flex;
  align-items: center;
  width: 16px;
  color: var(--text-disabled);
}

.qc-group-name {
  font-weight: 600;
  flex: 1;
}

.qc-group-count {
  color: var(--text-muted);
  font-weight: 400;
}

.qc-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all 0.12s ease;
  margin-bottom: 2px;
  user-select: none;
}

.qc-item.indented {
  padding-left: 26px;
}

.qc-item:hover {
  background: var(--bg-hover);
}

.qc-item.active {
  background: var(--accent-subtle);
  box-shadow: inset 0 0 0 1px var(--accent-dim);
}

.qc-item.active .qc-item-name {
  color: var(--accent);
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
