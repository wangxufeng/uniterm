# 快捷命令管理 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增快捷命令管理功能，用户可保存常用 shell 命令，按分组整理，通过侧边栏面板发送到当前活跃终端。

**Architecture:** 前端 Pinia store（`quickCommandStore`）通过 Wails 绑定调用 Go 后端读写 JSON 文件持久化。数据纳入已有同步机制。侧边栏重构为左侧图标切换栏 + 右侧面板区布局。新增 `QuickCommandsPanel` 和 `QuickCommandEditDialog` 两个 Vue 组件。

**Tech Stack:** Vue 3 + TypeScript + Pinia + Element Plus + Go + Wails v2

---

## 文件结构

| 文件 | 职责 | 新建/修改 |
|------|------|-----------|
| `backend/store/quick_commands_store.go` | Go 端读写 quickCommands.json | 新建 |
| `app.go` | 新增 Save/Load RPC，同步中加 quickCommands.json | 修改 |
| `backend/sync/sync_service.go` | 同步文件列表加入 quickCommands.json | 修改 |
| `frontend/src/stores/quickCommandStore.ts` | 前端 Pinia store，CRUD + 持久化 | 新建 |
| `frontend/src/components/QuickCommandsPanel.vue` | 侧边栏快捷命令面板 | 新建 |
| `frontend/src/components/QuickCommandEditDialog.vue` | 添加/编辑弹窗 | 新建 |
| `frontend/src/components/Sidebar.vue` | 重构布局，加切换按钮栏 | 修改 |
| `frontend/src/i18n/locales/*.json` (9 个) | 新增 quickCommands.* 翻译 | 修改 |

---

### Task 1: Go 后端 — QuickCommands 持久化

**Files:**
- Create: `backend/store/quick_commands_store.go`
- Modify: `app.go` (add ~30 lines)

- [ ] **Step 1: 创建 Go store 文件**

```go
// backend/store/quick_commands_store.go
package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const quickCommandsFileName = "quickCommands.json"

type QuickCommand struct {
	ID        string  `json:"id"`
	Name      string  `json:"name,omitempty"`
	Command   string  `json:"command"`
	GroupID   string  `json:"groupId,omitempty"`
	SortOrder int     `json:"sortOrder"`
}

type QuickCommandGroup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int    `json:"sortOrder"`
}

type QuickCommandData struct {
	Version  int                 `json:"version"`
	Groups   []QuickCommandGroup `json:"groups"`
	Commands []QuickCommand      `json:"commands"`
}

type QuickCommandsStore struct {
	configDir string
}

func NewQuickCommandsStore(configDir string) *QuickCommandsStore {
	return &QuickCommandsStore{configDir: configDir}
}

func (s *QuickCommandsStore) filePath() string {
	return filepath.Join(s.configDir, quickCommandsFileName)
}

func (s *QuickCommandsStore) Save(data QuickCommandData) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath(), bytes, 0600)
}

func (s *QuickCommandsStore) Load() (QuickCommandData, error) {
	bytes, err := os.ReadFile(s.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return QuickCommandData{Version: 1, Groups: []QuickCommandGroup{}, Commands: []QuickCommand{}}, nil
		}
		return QuickCommandData{}, err
	}
	var data QuickCommandData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return QuickCommandData{}, err
	}
	if data.Version == 0 {
		data.Version = 1
	}
	return data, nil
}
```

- [ ] **Step 2: 在 App 结构体中添加 quickCommandsStore 字段**

Read `app.go` 找到 `settingsStore` 字段声明处（约第 25-35 行），在其附近新增：

```go
quickCommandsStore *store.QuickCommandsStore
```

- [ ] **Step 3: 在 main() 或 App 初始化中创建 quickCommandsStore**

Read `app.go` 找到 `settingsStore` 初始化处（约第 65-80 行），在其附近新增：

```go
quickCommandsStore := store.NewQuickCommandsStore(appDir)
a.quickCommandsStore = quickCommandsStore
```

- [ ] **Step 4: 添加 SaveQuickCommands 和 LoadQuickCommands RPC 方法**

在 `app.go` 末尾（`SaveSettings` 附近）新增：

```go
func (a *App) SaveQuickCommands(data store.QuickCommandData) error {
	if a.quickCommandsStore == nil {
		return fmt.Errorf("quick commands store not initialized")
	}
	return a.quickCommandsStore.Save(data)
}

func (a *App) LoadQuickCommands() (store.QuickCommandData, error) {
	if a.quickCommandsStore == nil {
		return store.QuickCommandData{}, fmt.Errorf("quick commands store not initialized")
	}
	return a.quickCommandsStore.Load()
}
```

- [ ] **Step 5: 同步文件列表加入 quickCommands.json**

Read `backend/sync/sync_service.go` 找到 `[]string{"connections.json", "settings.json"}`（4 处：约 431, 463, 698, 708 行），每处修改为：

```go
[]string{"connections.json", "settings.json", "quickCommands.json"}
```

- [ ] **Step 6: 编译验证**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm && wails build -platform windows/amd64
```

Expected: 编译成功，无报错。

---

### Task 2: 前端 quickCommandStore

**Files:**
- Create: `frontend/src/stores/quickCommandStore.ts`

- [ ] **Step 1: 创建 quickCommandStore**

```typescript
// frontend/src/stores/quickCommandStore.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { SaveQuickCommands, LoadQuickCommands } from '../../wailsjs/go/main/App'

export interface QuickCommand {
  id: string
  name?: string
  command: string
  groupId?: string
  sortOrder: number
}

export interface QuickCommandGroup {
  id: string
  name: string
  sortOrder: number
}

export interface QuickCommandData {
  version: number
  groups: QuickCommandGroup[]
  commands: QuickCommand[]
}

let idCounter = 0
function genId(prefix: string): string {
  return `${prefix}-${Date.now()}-${++idCounter}`
}

export const useQuickCommandStore = defineStore('quickCommands', () => {
  const groups = ref<QuickCommandGroup[]>([])
  const commands = ref<QuickCommand[]>([])
  const loaded = ref(false)

  async function load() {
    if (loaded.value) return
    try {
      const data: QuickCommandData = await LoadQuickCommands()
      groups.value = data.groups || []
      commands.value = data.commands || []
    } catch (e) {
      console.error('Failed to load quick commands:', e)
      groups.value = []
      commands.value = []
    }
    loaded.value = true
  }

  async function save() {
    try {
      await SaveQuickCommands({
        version: 1,
        groups: JSON.parse(JSON.stringify(groups.value)),
        commands: JSON.parse(JSON.stringify(commands.value)),
      })
    } catch (e) {
      console.error('Failed to save quick commands:', e)
    }
  }

  function addGroup(name: string): QuickCommandGroup {
    const group: QuickCommandGroup = {
      id: genId('qcg'),
      name,
      sortOrder: groups.value.length,
    }
    groups.value.push(group)
    save()
    return group
  }

  function renameGroup(id: string, name: string) {
    const g = groups.value.find(x => x.id === id)
    if (g) { g.name = name; save() }
  }

  function deleteGroup(id: string, deleteCommands: boolean) {
    if (deleteCommands) {
      commands.value = commands.value.filter(c => c.groupId !== id)
    } else {
      commands.value.forEach(c => { if (c.groupId === id) c.groupId = undefined })
    }
    groups.value = groups.value.filter(g => g.id !== id)
    save()
  }

  function addCommand(name: string | undefined, command: string, groupId?: string): QuickCommand {
    const cmd: QuickCommand = {
      id: genId('qcc'),
      name: name || undefined,
      command,
      groupId,
      sortOrder: commands.value.filter(c => c.groupId === groupId).length,
    }
    commands.value.push(cmd)
    save()
    return cmd
  }

  function updateCommand(id: string, name: string | undefined, command: string, groupId?: string) {
    const c = commands.value.find(x => x.id === id)
    if (c) {
      c.name = name || undefined
      c.command = command
      c.groupId = groupId
      save()
    }
  }

  function deleteCommand(id: string) {
    commands.value = commands.value.filter(c => c.id !== id)
    save()
  }

  function getCommandsByGroup(groupId?: string): QuickCommand[] {
    return commands.value
      .filter(c => (c.groupId || undefined) === (groupId || undefined))
      .sort((a, b) => a.sortOrder - b.sortOrder)
  }

  return {
    groups, commands, loaded,
    load, save,
    addGroup, renameGroup, deleteGroup,
    addCommand, updateCommand, deleteCommand,
    getCommandsByGroup,
  }
})
```

- [ ] **Step 2: 构建前端验证**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm/frontend && npm run build
```

Expected: 构建成功（虽然 store 尚未被引用，但 TypeScript 编译应通过）。

---

### Task 3: i18n 翻译 Keys

**Files:**
- Modify: `frontend/src/i18n/locales/zh-CN.json`
- Modify: `frontend/src/i18n/locales/en.json`
- Modify: `frontend/src/i18n/locales/zh-TW.json`
- Modify: `frontend/src/i18n/locales/ja.json`
- Modify: `frontend/src/i18n/locales/ko.json`
- Modify: `frontend/src/i18n/locales/fr.json`
- Modify: `frontend/src/i18n/locales/de.json`
- Modify: `frontend/src/i18n/locales/es.json`
- Modify: `frontend/src/i18n/locales/ru.json`

- [ ] **Step 1: 在 zh-CN.json 末尾添加（最后一个 key 后加逗号）**

```json
  "quickCommands.title": "快捷命令",
  "quickCommands.addGroup": "添加分组",
  "quickCommands.addCommand": "添加命令",
  "quickCommands.editGroup": "编辑分组",
  "quickCommands.deleteGroup": "删除分组",
  "quickCommands.groupName": "分组名称",
  "quickCommands.renameGroup": "重命名分组",
  "quickCommands.deleteConfirm": "确定删除此命令？",
  "quickCommands.deleteGroupTitle": "删除分组",
  "quickCommands.deleteGroupDesc": "分组中的命令如何处理？",
  "quickCommands.moveToUngrouped": "移至未分组",
  "quickCommands.deleteCommands": "同时删除命令",
  "quickCommands.noGroup": "无分组",
  "quickCommands.command": "命令",
  "quickCommands.name": "名称",
  "quickCommands.namePlaceholder": "可选，为空时显示命令文本",
  "quickCommands.commandPlaceholder": "如 df -h",
  "quickCommands.run": "执行",
  "quickCommands.paste": "粘贴",
  "quickCommands.save": "保存",
  "quickCommands.cancel": "取消",
  "quickCommands.noActiveTerminal": "无活跃终端"
```

- [ ] **Step 2: 在 en.json 末尾添加**

```json
  "quickCommands.title": "Quick Commands",
  "quickCommands.addGroup": "Add Group",
  "quickCommands.addCommand": "Add Command",
  "quickCommands.editGroup": "Edit Group",
  "quickCommands.deleteGroup": "Delete Group",
  "quickCommands.groupName": "Group Name",
  "quickCommands.renameGroup": "Rename Group",
  "quickCommands.deleteConfirm": "Delete this command?",
  "quickCommands.deleteGroupTitle": "Delete Group",
  "quickCommands.deleteGroupDesc": "What to do with commands in this group?",
  "quickCommands.moveToUngrouped": "Move to ungrouped",
  "quickCommands.deleteCommands": "Delete commands",
  "quickCommands.noGroup": "No Group",
  "quickCommands.command": "Command",
  "quickCommands.name": "Name",
  "quickCommands.namePlaceholder": "Optional, command text shown if empty",
  "quickCommands.commandPlaceholder": "e.g. df -h",
  "quickCommands.run": "Run",
  "quickCommands.paste": "Paste",
  "quickCommands.save": "Save",
  "quickCommands.cancel": "Cancel",
  "quickCommands.noActiveTerminal": "No active terminal"
```

- [ ] **Step 3: 在其他语言文件中添加对应翻译**

zh-TW.json:
```json
  "quickCommands.title": "快捷命令",
  "quickCommands.addGroup": "新增群組",
  "quickCommands.addCommand": "新增命令",
  "quickCommands.editGroup": "編輯群組",
  "quickCommands.deleteGroup": "刪除群組",
  "quickCommands.groupName": "群組名稱",
  "quickCommands.renameGroup": "重新命名群組",
  "quickCommands.deleteConfirm": "確定刪除此命令？",
  "quickCommands.deleteGroupTitle": "刪除群組",
  "quickCommands.deleteGroupDesc": "群組中的命令如何處理？",
  "quickCommands.moveToUngrouped": "移至未分組",
  "quickCommands.deleteCommands": "同時刪除命令",
  "quickCommands.noGroup": "無群組",
  "quickCommands.command": "命令",
  "quickCommands.name": "名稱",
  "quickCommands.namePlaceholder": "可選，為空時顯示命令文字",
  "quickCommands.commandPlaceholder": "如 df -h",
  "quickCommands.run": "執行",
  "quickCommands.paste": "貼上",
  "quickCommands.save": "儲存",
  "quickCommands.cancel": "取消",
  "quickCommands.noActiveTerminal": "無活躍終端"
```

ja.json:
```json
  "quickCommands.title": "クイックコマンド",
  "quickCommands.addGroup": "グループ追加",
  "quickCommands.addCommand": "コマンド追加",
  "quickCommands.editGroup": "グループ編集",
  "quickCommands.deleteGroup": "グループ削除",
  "quickCommands.groupName": "グループ名",
  "quickCommands.renameGroup": "グループ名変更",
  "quickCommands.deleteConfirm": "このコマンドを削除しますか？",
  "quickCommands.deleteGroupTitle": "グループ削除",
  "quickCommands.deleteGroupDesc": "グループ内のコマンドをどうしますか？",
  "quickCommands.moveToUngrouped": "未分類に移動",
  "quickCommands.deleteCommands": "コマンドも削除",
  "quickCommands.noGroup": "グループなし",
  "quickCommands.command": "コマンド",
  "quickCommands.name": "名前",
  "quickCommands.namePlaceholder": "任意、空の場合はコマンドを表示",
  "quickCommands.commandPlaceholder": "例: df -h",
  "quickCommands.run": "実行",
  "quickCommands.paste": "貼り付け",
  "quickCommands.save": "保存",
  "quickCommands.cancel": "キャンセル",
  "quickCommands.noActiveTerminal": "アクティブな端末がありません"
```

ko.json:
```json
  "quickCommands.title": "빠른 명령",
  "quickCommands.addGroup": "그룹 추가",
  "quickCommands.addCommand": "명령 추가",
  "quickCommands.editGroup": "그룹 편집",
  "quickCommands.deleteGroup": "그룹 삭제",
  "quickCommands.groupName": "그룹 이름",
  "quickCommands.renameGroup": "그룹 이름 변경",
  "quickCommands.deleteConfirm": "이 명령을 삭제하시겠습니까?",
  "quickCommands.deleteGroupTitle": "그룹 삭제",
  "quickCommands.deleteGroupDesc": "그룹 내 명령을 어떻게 처리할까요?",
  "quickCommands.moveToUngrouped": "미분류로 이동",
  "quickCommands.deleteCommands": "명령도 삭제",
  "quickCommands.noGroup": "그룹 없음",
  "quickCommands.command": "명령",
  "quickCommands.name": "이름",
  "quickCommands.namePlaceholder": "선택 사항, 비어 있으면 명령 표시",
  "quickCommands.commandPlaceholder": "예: df -h",
  "quickCommands.run": "실행",
  "quickCommands.paste": "붙여넣기",
  "quickCommands.save": "저장",
  "quickCommands.cancel": "취소",
  "quickCommands.noActiveTerminal": "활성 터미널 없음"
```

fr.json:
```json
  "quickCommands.title": "Commandes rapides",
  "quickCommands.addGroup": "Ajouter un groupe",
  "quickCommands.addCommand": "Ajouter une commande",
  "quickCommands.editGroup": "Modifier le groupe",
  "quickCommands.deleteGroup": "Supprimer le groupe",
  "quickCommands.groupName": "Nom du groupe",
  "quickCommands.renameGroup": "Renommer le groupe",
  "quickCommands.deleteConfirm": "Supprimer cette commande ?",
  "quickCommands.deleteGroupTitle": "Supprimer le groupe",
  "quickCommands.deleteGroupDesc": "Que faire des commandes du groupe ?",
  "quickCommands.moveToUngrouped": "Déplacer hors groupe",
  "quickCommands.deleteCommands": "Supprimer les commandes",
  "quickCommands.noGroup": "Sans groupe",
  "quickCommands.command": "Commande",
  "quickCommands.name": "Nom",
  "quickCommands.namePlaceholder": "Optionnel, affiche la commande si vide",
  "quickCommands.commandPlaceholder": "ex: df -h",
  "quickCommands.run": "Exécuter",
  "quickCommands.paste": "Coller",
  "quickCommands.save": "Enregistrer",
  "quickCommands.cancel": "Annuler",
  "quickCommands.noActiveTerminal": "Aucun terminal actif"
```

de.json:
```json
  "quickCommands.title": "Schnellbefehle",
  "quickCommands.addGroup": "Gruppe hinzufügen",
  "quickCommands.addCommand": "Befehl hinzufügen",
  "quickCommands.editGroup": "Gruppe bearbeiten",
  "quickCommands.deleteGroup": "Gruppe löschen",
  "quickCommands.groupName": "Gruppenname",
  "quickCommands.renameGroup": "Gruppe umbenennen",
  "quickCommands.deleteConfirm": "Diesen Befehl löschen?",
  "quickCommands.deleteGroupTitle": "Gruppe löschen",
  "quickCommands.deleteGroupDesc": "Was soll mit den Befehlen in dieser Gruppe geschehen?",
  "quickCommands.moveToUngrouped": "In ungruppiert verschieben",
  "quickCommands.deleteCommands": "Befehle löschen",
  "quickCommands.noGroup": "Keine Gruppe",
  "quickCommands.command": "Befehl",
  "quickCommands.name": "Name",
  "quickCommands.namePlaceholder": "Optional, Befehl wird angezeigt wenn leer",
  "quickCommands.commandPlaceholder": "z.B. df -h",
  "quickCommands.run": "Ausführen",
  "quickCommands.paste": "Einfügen",
  "quickCommands.save": "Speichern",
  "quickCommands.cancel": "Abbrechen",
  "quickCommands.noActiveTerminal": "Kein aktives Terminal"
```

es.json:
```json
  "quickCommands.title": "Comandos rápidos",
  "quickCommands.addGroup": "Añadir grupo",
  "quickCommands.addCommand": "Añadir comando",
  "quickCommands.editGroup": "Editar grupo",
  "quickCommands.deleteGroup": "Eliminar grupo",
  "quickCommands.groupName": "Nombre del grupo",
  "quickCommands.renameGroup": "Renombrar grupo",
  "quickCommands.deleteConfirm": "¿Eliminar este comando?",
  "quickCommands.deleteGroupTitle": "Eliminar grupo",
  "quickCommands.deleteGroupDesc": "¿Qué hacer con los comandos del grupo?",
  "quickCommands.moveToUngrouped": "Mover a sin grupo",
  "quickCommands.deleteCommands": "Eliminar comandos",
  "quickCommands.noGroup": "Sin grupo",
  "quickCommands.command": "Comando",
  "quickCommands.name": "Nombre",
  "quickCommands.namePlaceholder": "Opcional, muestra el comando si está vacío",
  "quickCommands.commandPlaceholder": "ej: df -h",
  "quickCommands.run": "Ejecutar",
  "quickCommands.paste": "Pegar",
  "quickCommands.save": "Guardar",
  "quickCommands.cancel": "Cancelar",
  "quickCommands.noActiveTerminal": "Sin terminal activa"
```

ru.json:
```json
  "quickCommands.title": "Быстрые команды",
  "quickCommands.addGroup": "Добавить группу",
  "quickCommands.addCommand": "Добавить команду",
  "quickCommands.editGroup": "Редактировать группу",
  "quickCommands.deleteGroup": "Удалить группу",
  "quickCommands.groupName": "Имя группы",
  "quickCommands.renameGroup": "Переименовать группу",
  "quickCommands.deleteConfirm": "Удалить эту команду?",
  "quickCommands.deleteGroupTitle": "Удалить группу",
  "quickCommands.deleteGroupDesc": "Что делать с командами в группе?",
  "quickCommands.moveToUngrouped": "Переместить без группы",
  "quickCommands.deleteCommands": "Удалить команды",
  "quickCommands.noGroup": "Без группы",
  "quickCommands.command": "Команда",
  "quickCommands.name": "Имя",
  "quickCommands.namePlaceholder": "Необязательно, команда отображается если пусто",
  "quickCommands.commandPlaceholder": "напр. df -h",
  "quickCommands.run": "Выполнить",
  "quickCommands.paste": "Вставить",
  "quickCommands.save": "Сохранить",
  "quickCommands.cancel": "Отмена",
  "quickCommands.noActiveTerminal": "Нет активного терминала"
```

- [ ] **Step 4: 构建验证**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm/frontend && npm run build
```

Expected: 构建成功。JSON 语法正确。

---

### Task 4: 侧边栏改造 — 顶部切换标签

**Files:**
- Modify: `frontend/src/components/Sidebar.vue`

- [ ] **Step 1: 读取当前 Sidebar.vue 完整内容**

Read the full file to understand exact structure before making edits.

- [ ] **Step 2: 改造模板 — 在搜索框上方添加切换标签 + 条件面板**

在现有 `<div class="sidebar-header">` 下方（搜索框之前）插入切换标签栏，并用 `v-if` 控制连接面板和快捷命令面板的显示。

```html
<template>
  <div v-if="visible" class="sidebar" ref="sidebarEl" :style="{ width: Math.max(200, Math.min(600, sidebarWidth)) + 'px' }">
    <div class="resize-handle" @mousedown="onResizeMouseDown"></div>
    <div class="sidebar-header">
      <span class="header-label">{{ t('sidebar.title') }}</span>
      <button class="icon-btn" @click="emit('toggle')" :title="t('sidebar.collapse')"><X :size="14" /></button>
    </div>

    <!-- 顶部切换标签 -->
    <div class="sidebar-tabs">
      <button
        class="sidebar-tab"
        :class="{ active: activeView === 'connections' }"
        @click="activeView = 'connections'"
      >{{ t('sidebar.title') }}</button>
      <button
        class="sidebar-tab"
        :class="{ active: activeView === 'quickCommands' }"
        @click="activeView = 'quickCommands'"
      >{{ t('quickCommands.title') }}</button>
    </div>

    <!-- 连接面板 -->
    <template v-if="activeView === 'connections'">
      <div class="search-box">
        ... (现有搜索框完整保留)
      </div>
      <div class="connection-list" ref="listRef" @scroll="onListScroll">
        ... (现有连接列表完整保留)
      </div>
    </template>

    <!-- 快捷命令面板 -->
    <QuickCommandsPanel v-if="activeView === 'quickCommands'" />

    <!-- 所有现有弹窗保持原样 -->
    <ConnectionForm ... />
    <div v-show="contextMenu.visible" ...> ... </div>
    <el-dialog v-model="showDeleteGroupDialog" ...> ... </el-dialog>
    ...
  </div>
</template>
```

- [ ] **Step 3: 添加 script 修改**

在 `<script setup>` 中新增：

```typescript
import QuickCommandsPanel from './QuickCommandsPanel.vue'

const activeView = ref<'connections' | 'quickCommands'>('connections')
```

- [ ] **Step 4: 添加样式**

在 `<style scoped>` 末尾添加：

```css
.sidebar-tabs {
  display: flex;
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
}

.sidebar-tab {
  flex: 1;
  padding: 8px 12px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  background: transparent;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: all 0.15s;
  text-align: center;
}

.sidebar-tab:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.sidebar-tab.active {
  color: var(--accent-color);
  border-bottom-color: var(--accent-color);
}
```

- [ ] **Step 5: 构建验证**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm/frontend && npm run build
```

Expected: 构建失败（QuickCommandsPanel 尚未创建）。继续 Task 5。

---

### Task 5: QuickCommandEditDialog 组件

**Files:**
- Create: `frontend/src/components/QuickCommandEditDialog.vue`

- [ ] **Step 1: 创建对话框组件**

```vue
<template>
  <el-dialog
    v-model="visible"
    :title="editingId ? t('quickCommands.editCommand') : t('quickCommands.addCommand')"
    width="480px"
    :close-on-click-modal="false"
    @close="resetForm"
  >
    <el-form label-width="60px">
      <el-form-item :label="t('quickCommands.name')">
        <el-input
          v-model="formName"
          :placeholder="t('quickCommands.namePlaceholder')"
          maxlength="50"
        />
      </el-form-item>
      <el-form-item :label="t('quickCommands.group')">
        <el-select v-model="formGroupId" :placeholder="t('quickCommands.noGroup')" clearable>
          <el-option
            v-for="g in store.groups"
            :key="g.id"
            :label="g.name"
            :value="g.id"
          />
        </el-select>
      </el-form-item>
      <el-form-item :label="t('quickCommands.command')">
        <el-input
          v-model="formCommand"
          type="textarea"
          :rows="4"
          :placeholder="t('quickCommands.commandPlaceholder')"
          class="command-textarea"
        />
      </el-form-item>
    </el-form>

    <div v-if="errorMsg" class="form-error">{{ errorMsg }}</div>

    <template #footer>
      <el-button @click="visible = false">{{ t('quickCommands.cancel') }}</el-button>
      <el-button type="primary" :disabled="!formCommand.trim()" @click="handleSave">
        {{ t('quickCommands.save') }}
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useQuickCommandStore } from '../stores/quickCommandStore'
import { useI18n } from '../i18n'

const { t } = useI18n()
const store = useQuickCommandStore()

const props = defineProps<{
  modelValue: boolean
  editingId?: string       // 编辑模式下的命令 ID
  initialName?: string     // 编辑模式下的初始名称
  initialCommand?: string  // 编辑模式下的初始命令
  initialGroupId?: string  // 编辑模式下的初始分组
}>()

const emit = defineEmits<{
  'update:modelValue': [v: boolean]
}>()

const visible = computed({
  get: () => props.modelValue,
  set: (v) => emit('update:modelValue', v),
})

const formName = ref('')
const formCommand = ref('')
const formGroupId = ref<string | undefined>(undefined)
const errorMsg = ref('')

// 监听 visible 变化，初始化表单
import { watch } from 'vue'
watch(visible, (v) => {
  if (v) {
    formName.value = props.initialName || ''
    formCommand.value = props.initialCommand || ''
    formGroupId.value = props.initialGroupId || undefined
    errorMsg.value = ''
  }
})

function handleSave() {
  const cmd = formCommand.value.trim()
  if (!cmd) {
    errorMsg.value = t('quickCommands.commandRequired')
    return
  }
  if (props.editingId) {
    store.updateCommand(props.editingId, formName.value || undefined, cmd, formGroupId.value)
  } else {
    store.addCommand(formName.value || undefined, cmd, formGroupId.value)
  }
  visible.value = false
  resetForm()
}

function resetForm() {
  formName.value = ''
  formCommand.value = ''
  formGroupId.value = undefined
  errorMsg.value = ''
}
</script>

<style scoped>
.command-textarea :deep(textarea) {
  font-family: var(--font-mono, 'Consolas', 'Courier New', monospace);
}
.form-error {
  color: var(--danger-color, #f56c6c);
  font-size: 12px;
  margin-top: -8px;
  margin-bottom: 8px;
}
</style>
```

- [ ] **Step 2: 在 i18n 中添加缺失的 keys**

在 zh-CN.json 和 en.json 中添加：

```json
"quickCommands.editCommand": "编辑命令",
"quickCommands.group": "分组",
"quickCommands.commandRequired": "请输入命令"
```

(zh-CN)

```json
"quickCommands.editCommand": "Edit Command",
"quickCommands.group": "Group",
"quickCommands.commandRequired": "Command is required"
```

(en)

同步加其他 7 个语言文件。

- [ ] **Step 3: 构建验证**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm/frontend && npm run build
```

Expected: 构建成功（dialog 尚未被引用但独立编译应通过）。

---

### Task 6: QuickCommandsPanel 组件

**Files:**
- Create: `frontend/src/components/QuickCommandsPanel.vue`

- [ ] **Step 1: 创建面板组件**

```vue
<template>
  <div class="quick-commands-panel">
    <!-- 顶部操作栏 -->
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

    <!-- 命令列表 -->
    <div class="qc-list" ref="listRef">
      <!-- 分组渲染 -->
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

      <!-- 未分组命令 -->
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

      <!-- 空状态 -->
      <div v-if="store.commands.length === 0" class="qc-empty">
        {{ t('quickCommands.empty') }}
      </div>
    </div>

    <!-- 右键菜单: 命令 -->
    <div
      v-show="cmdContextMenu.visible"
      class="qc-context-menu"
      :style="{ left: cmdContextMenu.x + 'px', top: cmdContextMenu.y + 'px' }"
      @click.stop
    >
      <div class="menu-item" @click="editCommand(cmdContextMenu.cmd!)">{{ t('quickCommands.editCommand') }}</div>
      <div class="menu-item danger" @click="deleteCommand(cmdContextMenu.cmd!)">{{ t('quickCommands.deleteCommand') }}</div>
    </div>

    <!-- 右键菜单: 分组 -->
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

    <!-- 删除分组弹窗 -->
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

    <!-- 分组名称弹窗 (新建 + 重命名) -->
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

    <!-- 命令编辑弹窗 -->
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
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import {
  FolderPlus, Plus, Play, Clipboard,
  ChevronDown, ChevronRight, Zap
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

// 状态
const selectedId = ref<string | null>(null)
const hoveredId = ref<string | null>(null)
const expandedGroups = ref<Set<string>>(new Set())

// 右键菜单状态
const cmdContextMenu = ref<{ visible: boolean; x: number; y: number; cmd: QuickCommand | null }>({ visible: false, x: 0, y: 0, cmd: null })
const groupContextMenu = ref<{ visible: boolean; x: number; y: number; group: QuickCommandGroup | null }>({ visible: false, x: 0, y: 0, group: null })

// 删除分组弹窗
const deleteGroupDialogVisible = ref(false)
const deletingGroup = ref<QuickCommandGroup | null>(null)

// 分组名称弹窗
const groupNameDialogVisible = ref(false)
const groupNameInput = ref('')
const renamingGroup = ref<QuickCommandGroup | null>(null)

// 命令编辑弹窗
const editDialogVisible = ref(false)
const editingCmdId = ref<string | undefined>(undefined)
const editingCmdName = ref<string | undefined>(undefined)
const editingCmdCommand = ref('')
const editingCmdGroupId = ref<string | undefined>(undefined)

onMounted(async () => {
  await store.load()
  // 默认展开所有分组
  store.groups.forEach(g => expandedGroups.value.add(g.id))
  // 点击其他地方关闭右键菜单
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
  if (expandedGroups.value.has(id)) {
    expandedGroups.value.delete(id)
  } else {
    expandedGroups.value.add(id)
  }
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

  // Run mode: split by \n, append \n if missing at end
  let text = cmd.command
  if (!text.endsWith('\n')) {
    text += '\n'
  }
  const lines = text.split('\n').filter(l => l.length > 0)
  for (let i = 0; i < lines.length; i++) {
    SessionWrite(sid, lines[i] + '\n')
    if (i < lines.length - 1) {
      await new Promise(r => setTimeout(r, 100))
    }
  }
}

function runCommand(cmd: QuickCommand) {
  sendCommand(cmd, 'run')
}

function pasteCommand(cmd: QuickCommand) {
  sendCommand(cmd, 'paste')
}

// 右键菜单处理
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
  if (renamingGroup.value) {
    store.renameGroup(renamingGroup.value.id, name)
  } else {
    store.addGroup(name)
  }
  groupNameDialogVisible.value = false
}

function deleteGroupDialog(group: QuickCommandGroup) {
  deletingGroup.value = group
  deleteGroupDialogVisible.value = true
  groupContextMenu.value.visible = false
}

function doDeleteGroup(deleteCommands: boolean) {
  if (deletingGroup.value) {
    store.deleteGroup(deletingGroup.value.id, deleteCommands)
  }
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
```

- [ ] **Step 2: 添加缺失的 i18n keys**

```json
// zh-CN.json
"quickCommands.empty": "暂无快捷命令，点击右上角 + 添加",
"quickCommands.editCommand": "编辑命令",
"quickCommands.deleteCommand": "删除命令",
"quickCommands.group": "分组",
"quickCommands.commandRequired": "请输入命令"
```

```json
// en.json
"quickCommands.empty": "No quick commands. Click + to add one.",
"quickCommands.editCommand": "Edit Command",
"quickCommands.deleteCommand": "Delete Command",
"quickCommands.group": "Group",
"quickCommands.commandRequired": "Command is required"
```

同步其他 7 个语言。

- [ ] **Step 3: 构建前端**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm/frontend && npm run build
```

Expected: 构建成功。

---

### Task 7: 全局构建 + 验证

**Files:**
- Verify: all files compile and link

- [ ] **Step 1: 完整 wails 编译**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm/frontend && rm -rf dist node_modules/.vite && npm run build && cd .. && wails build -platform windows/amd64
```

Expected: 编译成功，生成 `build/bin/uniTerm.exe`。

- [ ] **Step 2: 启动验证**

```bash
cd c:/Users/Admin/Documents/Workspaces/uniTerm && wails dev
```

验证清单：
1. 侧边栏切换按钮 → 连接/快捷命令切换正常
2. 创建分组 → 面板中显示
3. 创建命令（有名称）→ 名称在上，命令在下浅色
4. 创建命令（无名称）→ 只显示命令，浅色
5. hover 命令项 → 显示 Run / Paste 按钮
6. 单击命令 → 选中高亮
7. 双击命令 → Run 发送到活跃终端并执行
8. 点击 Paste → 原样粘贴，不追加回车
9. 右键分组 → 重命名 / 删除
10. 右键命令 → 编辑 / 删除
11. 删除分组 → 选择移至未分组 或 同时删除命令
12. 重启应用 → 命令和分组持久化
